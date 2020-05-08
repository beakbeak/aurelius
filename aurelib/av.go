/*
Package aurelib is a thin wrapper around FFmpeg's audio decoding and encoding
functionality.

Raw audio data is produced by a Source (usually decoded from a compressed file
by FileSource), optionally resampled by a Resampler, then stored in a Fifo.
Fixed-size Frames containing raw audio data are pulled from the Fifo and then
passed to a Sink for encoding. A FileSink will write encoded data to disk, and a
BufferSink will store encoded data in memory.

Where noted, some objects are backed by heap-allocated C data structures and
must be destroyed with the Destroy method before being discarded.
*/
package aurelib

/*
#cgo pkg-config: libavformat libavcodec

#include <libavformat/avformat.h>
#include <libavcodec/avcodec.h>

static void
avRegisterAll() {
#if LIBAVFORMAT_VERSION_INT < AV_VERSION_INT(58, 7, 100)
    av_register_all();
#endif
}
*/
import "C"
import (
	"fmt"
	"time"
)

// A LogLevel represents the verbosity of FFmpeg's console logging. It
// corresponds to FFmpeg's AV_LOG_* constants.
type LogLevel int

// The following comments have been extracted from the FFmpeg documentation for
// the noted AV_LOG_* constants.
const (
	// "Print no output." (AV_LOG_QUIET)
	LogQuiet LogLevel = iota

	// "Something went really wrong and we will crash now." (AV_LOG_PANIC)
	LogPanic

	// "Something went wrong and recovery is not possible. For example, no
	// header was found for a format which depends on headers or an illegal
	// combination of parameters is used." (AV_LOG_FATAL)
	LogFatal

	// "Something went wrong and cannot losslessly be recovered. However, not
	// all future data is affected." (AV_LOG_ERROR)
	LogError

	// "Something somehow does not look correct. This may or may not lead to
	// problems. An example would be the use of '-vstrict -2'." (AV_LOG_WARNING)
	LogWarning

	// "Standard information." (AV_LOG_INFO)
	LogInfo

	// "Detailed information." (AV_LOG_VERBOSE)
	LogVerbose

	// "Stuff which is only useful for libav* developers." (AV_LOG_DEBUG)
	LogDebug

	// "Extremely verbose debugging, useful for libav* development."
	// (AV_LOG_TRACE)
	LogTrace
)

func init() {
	C.avRegisterAll()
	SetLogLevel(LogPanic)
}

// SetLogLevel controls the verbosity of FFmpeg's console logging. The default
// level is LogPanic.
func SetLogLevel(level LogLevel) {
	var avLevel C.int

	switch level {
	case LogQuiet:
		avLevel = C.AV_LOG_QUIET
	case LogPanic:
		avLevel = C.AV_LOG_PANIC
	case LogFatal:
		avLevel = C.AV_LOG_FATAL
	case LogError:
		avLevel = C.AV_LOG_ERROR
	case LogWarning:
		avLevel = C.AV_LOG_WARNING
	case LogInfo:
		avLevel = C.AV_LOG_INFO
	case LogVerbose:
		avLevel = C.AV_LOG_VERBOSE
	case LogDebug:
		avLevel = C.AV_LOG_DEBUG
	case LogTrace:
		avLevel = C.AV_LOG_TRACE
	}

	C.av_log_set_level(avLevel)
}

func avErr2Str(code C.int) string {
	var buffer [C.AV_ERROR_MAX_STRING_SIZE]C.char
	if C.av_strerror(code, &buffer[0], C.AV_ERROR_MAX_STRING_SIZE) < 0 {
		return "Unknown error"
	}
	return C.GoString(&buffer[0])
}

func (packet *C.AVPacket) init() {
	C.av_init_packet(packet)
	packet.data = nil
	packet.size = 0
}

func (ctx *C.AVCodecContext) channelLayout() C.int64_t {
	if ctx.channel_layout != 0 {
		return C.int64_t(ctx.channel_layout)
	}
	return C.av_get_default_channel_layout(ctx.channels)
}

// A StreamInfo contains properties of an audio stream.
type StreamInfo struct {
	SampleRate uint // The stream's sample rate in Hz.

	sampleFormat  int32
	channelLayout int64
}

func (ctx *C.AVCodecContext) streamInfo() StreamInfo {
	return StreamInfo{
		SampleRate:    uint(ctx.sample_rate),
		sampleFormat:  ctx.sample_fmt,
		channelLayout: int64(ctx.channelLayout()),
	}
}

func (info *StreamInfo) channelCount() C.int {
	return C.av_get_channel_layout_nb_channels(C.uint64_t(info.channelLayout))
}

func channelLayoutToString(channelLayout int64) string {
	return fmt.Sprintf("0x%x", channelLayout)
}

// ChannelLayout returns the stream's channel layout in a form that can be
// assigned to SinkOptions.ChannelLayout. It is not human-readable.
func (info *StreamInfo) ChannelLayout() string {
	return channelLayoutToString(info.channelLayout)
}

func sampleFormatToString(sampleFormat int32) string {
	return C.GoString(C.av_get_sample_fmt_name(C.enum_AVSampleFormat(sampleFormat)))
}

// SampleFormat returns an abbreviation of the stream's sample format ("s16",
// "flt", "u8p", etc.). It can be assigned to SinkOptions.SampleFormat.
//
// See FFmpeg's documentation for AVSampleFormat for a full list of formats.
func (info *StreamInfo) SampleFormat() string {
	return sampleFormatToString(info.sampleFormat)
}

// A Frame contains unencoded audio data.
//
// It is backed by a heap-allocated C data structure, so it must be destroyed
// with Destroy or passed to a function that takes ownership, such as
// Sink.Encode.
type Frame struct {
	frame *C.AVFrame

	Size uint // The number of samples contained in the frame.
}

// Destroy frees C heap memory used by the Frame and sets Size to 0.
func (frame Frame) Destroy() {
	if frame.frame != nil {
		C.av_frame_free(&frame.frame)
	}
	frame.Size = 0
}

// IsEmpty returns true if the Frame contains no audio data.
func (frame Frame) IsEmpty() bool {
	return frame.frame == nil || frame.Size == 0
}

func durationToTimeBase(
	duration time.Duration,
	timeBase C.AVRational,
) C.int64_t {
	// (duration / time.Second) / timeBase
	return C.int64_t(float64(duration) / (float64(time.Second) * float64(C.av_q2d(timeBase))))
}

func durationFromTimeBase(
	duration C.int64_t,
	timeBase C.AVRational,
) time.Duration {
	// (duration * timeBase) * time.Second
	return time.Duration(float64(duration) * (float64(time.Second) * float64(C.av_q2d(timeBase))))
}
