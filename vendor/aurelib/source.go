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
	"fmt"
	"math"
	"unsafe"
)

type ReplayGainMode int

const (
	ReplayGainTrack ReplayGainMode = iota
	ReplayGainAlbum
)

type sourceBase struct {
	tags map[string]string

	formatCtx *C.struct_AVFormatContext
	codecCtx  *C.struct_AVCodecContext
	stream    *C.struct_AVStream

	frame *C.struct_AVFrame
}

type FileSource struct {
	sourceBase
	Path string
}

type Source interface {
	Destroy()

	// ReplayGain returns the [0,1] volume scale factor that should be applied
	// based on the audio stream's ReplayGain metadata and the supplied
	// arguments.
	ReplayGain(
		mode ReplayGainMode,
		preventClipping bool,
	) float64

	// Decode transfers an encoded packet from the input to the decoder.
	// It must be followed by one or more calls to ReceiveFrame.
	// The return value following an error will be true if the error is
	// recoverable and Decode() can be safely called again.
	Decode() (error, bool /*recoverable*/)

	// ReceiveFrame receives a decoded frame from the decoder.
	// * If it returns ReceiveFrameCopyAndCallAgain, it must be followed by a
	//   call to CopyFrame and then called again.
	// * If it returns ReceiveFrameEmpty, there was no frame to be received.
	// * If it returns ReceiveFrameEof, all frames from this Source have been
	//   decoded and received.
	ReceiveFrame() (ReceiveFrameStatus, error)

	// FrameSize returns the number of samples in the last fram received by a
	// call to ReceiveFrame.
	FrameSize() uint

	// CopyFrame copies the data received by ReceiveFrame to the supplied FIFO,
	// resampling it with the supplied Resampler if it is not nil.
	// CopyFrame may be called multiple times to copy the data to multiple
	// FIFOs.
	CopyFrame(
		fifo *Fifo,
		rs *Resampler,
	) error

	SampleRate() int
	Tags() map[string]string

	codecContext() *C.struct_AVCodecContext
}

func (src *sourceBase) Destroy() {
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

func NewFileSource(path string) (*FileSource, error) {
	success := false
	src := FileSource{Path: path}
	defer func() {
		if !success {
			src.Destroy()
		}
	}()

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	if err := C.avformat_open_input(&src.formatCtx, cPath, nil, nil); err < 0 {
		return nil, fmt.Errorf("failed to open file: %v", avErr2Str(err))
	}

	if err := src.init(); err != nil {
		return nil, err
	}
	success = true
	return &src, nil
}

func (src *sourceBase) init() error {
	// gather streams
	if err := C.avformat_find_stream_info(src.formatCtx, nil); err < 0 {
		return fmt.Errorf("failed to find stream info: %v", avErr2Str(err))
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
		return fmt.Errorf("no audio streams")
	}

	codec := C.avcodec_find_decoder(src.stream.codecpar.codec_id)
	if codec == nil {
		return fmt.Errorf("failed to find decoder")
	}

	if src.codecCtx = C.avcodec_alloc_context3(codec); src.codecCtx == nil {
		return fmt.Errorf("failed to allocate decoding context")
	}

	if err := C.avcodec_parameters_to_context(src.codecCtx, src.stream.codecpar); err < 0 {
		return fmt.Errorf("failed to copy codec parameters: %v", avErr2Str(err))
	}

	if err := C.avcodec_open2(src.codecCtx, codec, nil); err < 0 {
		return fmt.Errorf("failed to open decoder: %v", avErr2Str(err))
	}

	if src.frame = C.av_frame_alloc(); src.frame == nil {
		return fmt.Errorf("failed to allocate input frame")
	}

	src.tags = make(map[string]string)

	gatherTagsFromDict := func(dict *C.struct_AVDictionary) {
		var entry *C.struct_AVDictionaryEntry
		for {
			if entry = C.av_dict_get(
				dict, C.strEmpty(), entry, C.AV_DICT_IGNORE_SUFFIX,
			); entry != nil {
				src.tags[C.GoString(entry.key)] = C.GoString(entry.value)
			} else {
				break
			}
		}
	}
	gatherTagsFromDict(src.formatCtx.metadata)
	gatherTagsFromDict(src.stream.metadata)

	return nil
}

func (src *sourceBase) codecContext() *C.struct_AVCodecContext {
	return src.codecCtx
}

func (src *FileSource) DumpFormat() {
	cPath := C.CString(src.Path)
	defer C.free(unsafe.Pointer(cPath))
	C.av_dump_format(src.formatCtx, 0, cPath, 0)
}

func (src *sourceBase) ReplayGain(
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
func (src *sourceBase) Decode() (error, bool /*recoverable*/) {
	var packet C.AVPacket
	packet.init()

	if err := C.av_read_frame(src.formatCtx, &packet); err == C.avErrorEOF() {
		// return?
	} else if err < 0 {
		return fmt.Errorf("failed to read frame: %v", avErr2Str(err)), false
	}
	defer C.av_packet_unref(&packet)

	if err := C.avcodec_send_packet(src.codecCtx, &packet); err < 0 {
		return fmt.Errorf("failed to send packet to decoder: %v", avErr2Str(err)), true
	}
	return nil, false
}

type ReceiveFrameStatus int

const (
	ReceiveFrameEmpty ReceiveFrameStatus = iota
	ReceiveFrameCopyAndCallAgain
	ReceiveFrameEof
)

func (src *sourceBase) ReceiveFrame() (ReceiveFrameStatus, error) {
	// calls av_frame_unref()
	if err := C.avcodec_receive_frame(src.codecCtx, src.frame); err == C.avErrorEAGAIN() {
		return ReceiveFrameEmpty, nil
	} else if err == C.avErrorEOF() {
		return ReceiveFrameEof, nil
	} else if err < 0 {
		return ReceiveFrameEmpty, fmt.Errorf("%s", avErr2Str(err))
	}
	return ReceiveFrameCopyAndCallAgain, nil
}

func (src *sourceBase) FrameSize() uint {
	if src.frame != nil {
		return uint(src.frame.nb_samples)
	}
	return 0
}

func (src *sourceBase) CopyFrame(
	fifo *Fifo,
	rs *Resampler,
) error {
	if rs != nil {
		if _, err := rs.convert(src.frame.extended_data, src.frame.nb_samples, fifo); err != nil {
			return err
		}
	} else if fifo.write(
		(*unsafe.Pointer)(unsafe.Pointer(src.frame.extended_data)), src.frame.nb_samples,
	) < src.frame.nb_samples {
		return fmt.Errorf("failed to write data to FIFO")
	}
	return nil
}

func (src *sourceBase) SampleRate() int {
	return int(src.codecCtx.sample_rate)
}

func (src *sourceBase) Tags() map[string]string {
	return src.tags
}
