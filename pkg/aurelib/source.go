package aurelib

/*
#cgo pkg-config: libavformat libavcodec libavutil

#include <libavformat/avformat.h>
#include <libavutil/replaygain.h>
#include <stdlib.h>

static int
avErrorEOF() {
	return AVERROR_EOF;
}

static int
avErrorEAGAIN() {
	return AVERROR(EAGAIN);
}

static char const*
strEmpty() {
	return "";
}
*/
import "C"
import (
	"fmt"
	"math"
	"regexp"
	"sb/aurelius/internal/maputil"
	"strconv"
	"time"
	"unsafe"
)

// A Source produces raw audio data to be consumed by a Sink.
type Source interface {
	// Destroy frees any resources held by the Source so that it may be
	// discarded.
	Destroy()

	// ReplayGain returns the volume scale factor that should be applied based
	// on the audio stream's ReplayGain metadata and the supplied arguments.
	//
	// If preventClipping is true and peak volume information is available, the
	// returned value will be clamped such that the scaled peak volume does not
	// exceed the maximum representable value.
	ReplayGain(
		mode ReplayGainMode,
		preventClipping bool,
	) float64

	// Tags returns a string map containing metadata describing the Source.
	// Common keys include "artist", "album", "title", "track", and "composer".
	Tags() map[string]string

	// StreamInfo returns an object describing the format of audio data produced
	// by the Source.
	StreamInfo() StreamInfo

	// Duration returns the duration of the audio stream.
	Duration() time.Duration

	// Seek causes streaming to continue from the given offset relative to the
	// beginning of the audio stream.
	SeekTo(offset time.Duration) error

	// Decode transfers an encoded packet from the input to the decoder. It must
	// be followed by one or more calls to ReceiveFrame.
	//
	// When a non-nil error is returned, the 'recoverable' return value will be
	// true if the error is recoverable and Decode can be safely called again.
	Decode() (err error, recoverable bool)

	// ReceiveFrame receives a decoded frame from the decoder.
	//
	// The returned ReceiveFrameStatus indicates how the code should proceed
	// after the call to ReceiveFrame.
	ReceiveFrame() (ReceiveFrameStatus, error)

	// FrameSize returns the number of samples in the last frame received by a
	// call to ReceiveFrame.
	FrameSize() uint

	// FrameStartTime returns the stream time offset of the start of the last
	// frame received by a call to ReceiveFrame.
	FrameStartTime() time.Duration

	// CopyFrame copies the data received by ReceiveFrame to the supplied Fifo.
	//
	// CopyFrame and ResampleFrame may be called multiple times to supply data
	// to multiple Fifos.
	CopyFrame(fifo *Fifo) error

	// ResampleFrame resamples the data received by ReceiveFrame and passes it
	// to the supplied Fifo.
	//
	// CopyFrame and ResampleFrame may be called multiple times to supply data
	// to multiple Fifos.
	ResampleFrame(
		rs *Resampler,
		fifo *Fifo,
	) error
}

// ReplayGainMode indicates which set of ReplayGain data to use in volume
// adjustment.
type ReplayGainMode int

const (
	ReplayGainTrack ReplayGainMode = iota // ReplayGain calculated per-track.
	ReplayGainAlbum                       // ReplayGain calculated for an entire album.
)

// A ReceiveFrameStatus is the result of a call to Source.ReceiveFrame.
type ReceiveFrameStatus int

const (
	// ReceiveFrameEmpty indicates that there was no frame to be received yet.
	ReceiveFrameEmpty ReceiveFrameStatus = iota

	// ReceiveFrameCopyAndCallAgain indicates that ReceiveFrame must be followed
	// by a call to CopyFrame or ResampleFrame before being called again.
	ReceiveFrameCopyAndCallAgain

	// ReceiveFrameEof indicates that all frames from the Source have been
	// decoded and received.
	ReceiveFrameEof
)

type sourceBase struct {
	tags map[string]string

	formatCtx *C.AVFormatContext
	codecCtx  *C.AVCodecContext
	stream    *C.AVStream

	frame *C.AVFrame
}

// A FileSource decodes audio data from a file stored in the local filesystem.
type FileSource struct {
	sourceBase

	Path string // The path passed to NewFileSource.
}

// Destroy frees any resources held by the Source so that it may be discarded.
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

// NewFileSource creates a new FileSource that will read audio data from the
// file specified by the path argument.
//
// The file may be in any format supported by FFmpeg, including formats that
// also include video data. If the file contains multiple audio streams, the
// first one reported by FFmpeg will be used.
//
// The Source is backed by a heap-allocated C data structure, so it must be
// destroyed with Destroy before it is discarded.
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
		streams := (*[1 << 30]*C.AVStream)(unsafe.Pointer(src.formatCtx.streams))[:streamCount:streamCount]
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

	gatherTagsFromDict := func(dict *C.AVDictionary) {
		var entry *C.AVDictionaryEntry
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

// DumpFormat logs information about the audio format with level LogInfo.
func (src *FileSource) DumpFormat() {
	cPath := C.CString(src.Path)
	defer C.free(unsafe.Pointer(cPath))
	C.av_dump_format(src.formatCtx, 0, cPath, 0)
}

func volumeFromGain(gain float64) float64 {
	return math.Pow(10, gain/20.)
}

func clampVolumeToPeak(
	volume float64,
	peak float64,
) float64 {
	invPeak := 1. / peak
	if volume > invPeak {
		volume = invPeak
	}
	return volume
}

func (src *sourceBase) replayGainFromSideData(
	mode ReplayGainMode,
	preventClipping bool,
) (float64, bool /*ok*/) {
	var data *C.AVPacketSideData
	for i := C.int(0); i < src.stream.nb_side_data; i++ {
		address := uintptr(unsafe.Pointer(src.stream.side_data))
		address += uintptr(i) * unsafe.Sizeof(*src.stream.side_data)
		localData := (*C.AVPacketSideData)(unsafe.Pointer(address))

		if localData._type == C.AV_PKT_DATA_REPLAYGAIN {
			data = localData
			break
		}
	}
	if data == nil {
		return 1., false
	}

	gainData := (*C.AVReplayGain)(unsafe.Pointer(data.data))

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
		return 1., false
	}

	gain /= 100000.
	peak /= 100000.

	volume := volumeFromGain(gain)
	if preventClipping && peakValid {
		volume = clampVolumeToPeak(volume, peak)
	}
	return volume, true
}

var reGain = regexp.MustCompile(`^([^ ]+) dB$`)

func (src *sourceBase) replayGainFromTags(
	mode ReplayGainMode,
	preventClipping bool,
) (float64, bool /*ok*/) {
	var gainTags, peakTags []string

	if mode == ReplayGainAlbum {
		gainTags = []string{"replaygain_album_gain", "replaygain_track_gain", "replaygain_gain"}
		peakTags = []string{"replaygain_album_peak", "replaygain_track_peak", "replaygain_peak"}
	} else {
		gainTags = []string{"replaygain_track_gain", "replaygain_album_gain", "replaygain_gain"}
		peakTags = []string{"replaygain_track_peak", "replaygain_album_peak", "replaygain_peak"}
	}

	var gain, peak float64
	var gainValid, peakValid bool

	lowerCaseTags := maputil.LowerCaseKeys(src.tags)

	for _, gainTag := range gainTags {
		if gainString, ok := lowerCaseTags[gainTag]; ok {
			if match := reGain.FindStringSubmatch(gainString); match != nil {
				var err error
				if gain, err = strconv.ParseFloat(match[1], 64); err == nil {
					gainValid = true
					break
				}
			}
		}
	}

	if !gainValid {
		return 1., false
	}

	for _, peakTag := range peakTags {
		if peakString, ok := lowerCaseTags[peakTag]; ok {
			var err error
			if peak, err = strconv.ParseFloat(peakString, 64); err == nil {
				peakValid = true
				break
			}
		}
	}

	volume := volumeFromGain(gain)
	if preventClipping && peakValid {
		volume = clampVolumeToPeak(volume, peak)
	}
	return volume, true
}

// ReplayGain returns the volume scale factor that should be applied based on
// the audio stream's ReplayGain metadata and the supplied arguments.
//
// If preventClipping is true and peak volume information is available, the
// returned value will be clamped such that the scaled peak volume does not
// exceed the maximum representable value.
func (src *sourceBase) ReplayGain(
	mode ReplayGainMode,
	preventClipping bool,
) float64 {
	if volume, ok := src.replayGainFromSideData(mode, preventClipping); ok {
		return volume
	}
	if volume, ok := src.replayGainFromTags(mode, preventClipping); ok {
		return volume
	}
	return 1.
}

// Decode transfers an encoded packet from the input to the decoder. It must be
// followed by one or more calls to ReceiveFrame.
//
// When a non-nil error is returned, the 'recoverable' return value will be true
// if the error is recoverable and Decode can be safely called again.
func (src *sourceBase) Decode() (err error, recoverable bool) {
	var packet C.AVPacket
	initPacket(&packet)

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

// ReceiveFrame receives a decoded frame from the decoder.
//
// The returned ReceiveFrameStatus indicates how the code should proceed after
// the call to ReceiveFrame.
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

// FrameSize returns the number of samples in the last frame received by a call
// to ReceiveFrame.
func (src *sourceBase) FrameSize() uint {
	if src.frame != nil {
		return uint(src.frame.nb_samples)
	}
	return 0
}

// FrameStartTime returns the stream time offset of the start of the last
// frame received by a call to ReceiveFrame.
func (src *sourceBase) FrameStartTime() time.Duration {
	if src.frame != nil {
		return durationFromTimeBase(src.frame.pts, src.stream.time_base)
	}
	return 0
}

// CopyFrame copies the data received by ReceiveFrame to the supplied Fifo.
//
// CopyFrame and ResampleFrame may be called multiple times to supply data to
// multiple Fifos.
func (src *sourceBase) CopyFrame(
	fifo *Fifo,
) error {
	if fifo.write(
		(*unsafe.Pointer)(unsafe.Pointer(src.frame.extended_data)), src.frame.nb_samples,
	) < src.frame.nb_samples {
		return fmt.Errorf("failed to write data to FIFO")
	}
	return nil
}

// ResampleFrame resamples the data received by ReceiveFrame and passes it to
// the supplied Fifo.
//
// CopyFrame and ResampleFrame may be called multiple times to supply data to
// multiple Fifos.
func (src *sourceBase) ResampleFrame(
	rs *Resampler,
	fifo *Fifo,
) error {
	if _, err := rs.resample(src.frame.extended_data, src.frame.nb_samples, fifo); err != nil {
		return err
	}
	return nil
}

// Tags returns a string map containing metadata describing the Source. Common
// keys include "artist", "album", "title", "track", and "composer".
func (src *sourceBase) Tags() map[string]string {
	return src.tags
}

// StreamInfo returns an object describing the format of audio data produced by
// the Source.
func (src *sourceBase) StreamInfo() StreamInfo {
	return streamInfoFromCodecContext(src.codecCtx)
}

// Duration returns the duration of the audio stream.
func (src *sourceBase) Duration() time.Duration {
	if src.formatCtx.duration <= 0 {
		return 0
	}
	return durationFromTimeBase(src.formatCtx.duration, C.av_make_q(1, C.AV_TIME_BASE))
}

// Seek causes streaming to continue from the given offset relative to the
// beginning of the audio stream.
func (src *sourceBase) SeekTo(offset time.Duration) error {
	streamOffset := durationToTimeBase(offset, src.stream.time_base)
	if err := C.av_seek_frame(src.formatCtx, src.stream.index, streamOffset, C.int(0)); err < 0 {
		return fmt.Errorf("unknown error")
	}
	return nil
}
