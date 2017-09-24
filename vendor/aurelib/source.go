package aurelib

/*
#cgo pkg-config: libavformat libavcodec libavutil

#include <libavformat/avformat.h>
#include <libavutil/replaygain.h>
#include <stdlib.h>

static int avErrorEOF() {
	return AVERROR_EOF;
}

static int avErrorEAGAIN() {
	return AVERROR(EAGAIN);
}

static char const* strEmpty() {
	return "";
}
*/
import "C"
import (
	"errors"
	"fmt"
	"math"
	"unsafe"
)

type ReplayGainMode int

const (
	ReplayGainTrack ReplayGainMode = iota
	ReplayGainAlbum
)

type Source struct {
	Path string
	Tags map[string]string

	formatCtx *C.struct_AVFormatContext
	codecCtx  *C.struct_AVCodecContext
	stream    *C.struct_AVStream

	frame *C.struct_AVFrame
}

func (src *Source) Destroy() {
	if src.frame != nil {
		C.av_frame_free(&src.frame)
	}
	if src.codecCtx != nil {
		C.avcodec_free_context(&src.codecCtx)
	}
	if src.formatCtx != nil {
		C.avformat_close_input(&src.formatCtx)
	}
}

func NewFileSource(path string) (*Source, error) {
	success := false
	src := Source{Path: path}
	defer func() {
		if !success {
			src.Destroy()
		}
	}()

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// open file
	if err := C.avformat_open_input(&src.formatCtx, cPath, nil, nil); err < 0 {
		return nil, fmt.Errorf("failed to open file: %v", avErr2Str(err))
	}

	// gather streams
	if err := C.avformat_find_stream_info(src.formatCtx, nil); err < 0 {
		return nil, fmt.Errorf("failed to find stream info: %v", avErr2Str(err))
	}

	// find first audio stream
	{
		streamCount := src.formatCtx.nb_streams
		streams := (*[1 << 30]*C.struct_AVStream)(unsafe.Pointer(src.formatCtx.streams))[:streamCount:streamCount]
		for _, stream := range streams {
			if stream.codecpar.codec_type == C.AVMEDIA_TYPE_AUDIO {
				src.stream = stream
				break
			}
		}
	}
	if src.stream == nil {
		return nil, errors.New("no audio streams")
	}

	codec := C.avcodec_find_decoder(src.stream.codecpar.codec_id)
	if codec == nil {
		return nil, errors.New("failed to find decoder")
	}

	if src.codecCtx = C.avcodec_alloc_context3(codec); src.codecCtx == nil {
		return nil, errors.New("failed to allocate decoding context")
	}

	if err := C.avcodec_parameters_to_context(src.codecCtx, src.stream.codecpar); err < 0 {
		return nil, fmt.Errorf("failed to copy codec parameters: %v", avErr2Str(err))
	}

	if err := C.avcodec_open2(src.codecCtx, codec, nil); err < 0 {
		return nil, fmt.Errorf("failed to open decoder: %v", avErr2Str(err))
	}

	src.Tags = make(map[string]string)

	gatherTagsFromDict := func(dict *C.struct_AVDictionary) {
		var entry *C.struct_AVDictionaryEntry
		for {
			if entry = C.av_dict_get(
				dict, C.strEmpty(), entry, C.AV_DICT_IGNORE_SUFFIX,
			); entry != nil {
				src.Tags[C.GoString(entry.key)] = C.GoString(entry.value)
			} else {
				break
			}
		}
	}
	gatherTagsFromDict(src.formatCtx.metadata)
	gatherTagsFromDict(src.stream.metadata)

	success = true
	return &src, nil
}

func (src *Source) DumpFormat() {
	cPath := C.CString(src.Path)
	defer C.free(unsafe.Pointer(cPath))
	C.av_dump_format(src.formatCtx, 0, cPath, 0)
}

// ReplayGain returns the [0,1] volume scale factor that should be applied
// based on the audio stream's ReplayGain metadata and the supplied arguments.
func (src *Source) ReplayGain(
	mode ReplayGainMode,
	preventClipping bool,
) float64 {
	var data *C.struct_AVPacketSideData
	for i := C.int(0); i < src.stream.nb_side_data; i++ {
		address := uintptr(unsafe.Pointer(src.stream.side_data))
		address += uintptr(i) * unsafe.Sizeof(*src.stream.side_data)
		localData := (*C.struct_AVPacketSideData)(unsafe.Pointer(address))

		if localData._type == C.AV_PKT_DATA_REPLAYGAIN {
			data = localData
			break
		}
	}
	if data == nil {
		return 1.
	}

	gainData := (*C.struct_AVReplayGain)(unsafe.Pointer(data.data))

	var gain, peak float64
	var gainValid, peakValid bool

	if gainData.track_gain != C.INT32_MIN {
		gain = float64(gainData.track_gain)
		gainValid = true
	}
	if gainData.track_peak != 0 {
		peak = float64(gainData.track_peak)
		peakValid = true
	}

	if (mode == ReplayGainAlbum || !gainValid) && gainData.album_gain != C.INT32_MIN {
		gain = float64(gainData.album_gain)
		gainValid = true
	}
	if (mode == ReplayGainAlbum || !peakValid) && gainData.album_peak != 0 {
		peak = float64(gainData.album_peak)
		peakValid = true
	}

	if !gainValid {
		return 1.
	}

	gain /= 100000.
	peak /= 100000.

	volume := math.Pow(10, gain/20.)
	if preventClipping && peakValid {
		invPeak := 1. / peak
		if volume > invPeak {
			volume = invPeak
		}
	}
	return volume
}

func (src *Source) Decode(
	fifo *Fifo,
	rs *Resampler,
) (bool /*finished*/, error) {
	if src.frame == nil {
		if src.frame = C.av_frame_alloc(); src.frame == nil {
			return false, fmt.Errorf("failed to allocate input frame")
		}
	}
	defer C.av_frame_unref(src.frame)

	var packet C.AVPacket
	packet.init()

	if err := C.av_read_frame(src.formatCtx, &packet); err == C.avErrorEOF() {
		// return?
	} else if err < 0 {
		return false, fmt.Errorf("failed to read frame: %v", avErr2Str(err))
	}
	defer C.av_packet_unref(&packet)

	if err := C.avcodec_send_packet(src.codecCtx, &packet); err < 0 {
		return false, fmt.Errorf("failed to send packet to decoder: %v", avErr2Str(err))
	}

	for {
		// calls av_frame_unref()
		if err := C.avcodec_receive_frame(src.codecCtx, src.frame); err == C.avErrorEAGAIN() {
			return false, nil
		} else if err == C.avErrorEOF() {
			return true, nil
		} else if err < 0 {
			return false, fmt.Errorf("failed to receive frame from decoder: %v", avErr2Str(err))
		}

		if rs != nil {
			if _, err := rs.convert(src.frame.extended_data, src.frame.nb_samples, fifo); err != nil {
				return false, err
			}
		} else if fifo.write(
			(*unsafe.Pointer)(unsafe.Pointer(src.frame.extended_data)), src.frame.nb_samples,
		) < src.frame.nb_samples {
			return false, fmt.Errorf("failed to write data to FIFO")
		}
	}
}
