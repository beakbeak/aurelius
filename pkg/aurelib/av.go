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
#include <libavutil/channel_layout.h>

#define LOG_BUFFER_SIZE 2048

extern void logMessage(int level, char const* buffer);

static void
avRegisterAll() {
#if LIBAVFORMAT_VERSION_INT < AV_VERSION_INT(58, 7, 100)
	av_register_all();
#endif
}

static void
logCallback(
	void*		ptr,
	int			level,
	char const*	format,
	va_list		format_args)
{
	char buffer[LOG_BUFFER_SIZE];
	int print_prefix = 1;

	av_log_format_line(ptr, level, format, format_args, buffer, LOG_BUFFER_SIZE, &print_prefix);
	logMessage(level, buffer);
}

static void
setLogCallback() {
	av_log_set_callback(logCallback);
}
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

func init() {
	C.avRegisterAll()
	C.setLogCallback()
}

func avErr2Str(code C.int) string {
	var buffer [C.AV_ERROR_MAX_STRING_SIZE]C.char
	if C.av_strerror(code, &buffer[0], C.AV_ERROR_MAX_STRING_SIZE) < 0 {
		return "Unknown error"
	}
	return C.GoString(&buffer[0])
}

func channelLayoutToString(layout *C.AVChannelLayout) string {
	length := C.av_channel_layout_describe(layout, nil, 0)
	buffer := make([]C.char, length)
	C.av_channel_layout_describe(layout, &buffer[0], C.size_t(length))
	return C.GoString(&buffer[0])
}

func channelLayoutFromString(layoutStr string, layout *C.AVChannelLayout) error {
	layoutCstr := C.CString(layoutStr)
	defer C.free(unsafe.Pointer(layoutCstr))
	if err := C.av_channel_layout_from_string(layout, layoutCstr); err < 0 {
		return fmt.Errorf("%s", avErr2Str(err))
	}
	return nil
}

func withChannelLayout(layoutStr string, f func(layout *C.AVChannelLayout)) {
	layout := C.AVChannelLayout{}
	defer C.av_channel_layout_uninit(&layout)
	if err := channelLayoutFromString(layoutStr, &layout); err != nil {
		logger.Log(LogError, fmt.Sprintf("failed to convert channel layout: %v", err))
		C.av_channel_layout_default(&layout, 2)
	}
	f(&layout)
}

// A StreamInfo contains properties of an audio stream.
type StreamInfo struct {
	SampleRate uint // The stream's sample rate in Hz.

	sampleFormat  int32
	channelLayout string
}

func streamInfoFromCodecContext(ctx *C.AVCodecContext) StreamInfo {
	return StreamInfo{
		SampleRate:    uint(ctx.sample_rate),
		sampleFormat:  ctx.sample_fmt,
		channelLayout: channelLayoutToString(&ctx.ch_layout),
	}
}

func (info *StreamInfo) channelCount() C.int {
	count := C.int(0)
	withChannelLayout(info.channelLayout, func(layout *C.AVChannelLayout) {
		count = layout.nb_channels
	})
	return count
}

// ChannelLayout returns the stream's channel layout in a form that can be
// assigned to SinkConfig.ChannelLayout. It is not human-readable.
func (info *StreamInfo) ChannelLayout() string {
	return info.channelLayout
}

func sampleFormatToString(sampleFormat int32) string {
	return C.GoString(C.av_get_sample_fmt_name(C.enum_AVSampleFormat(sampleFormat)))
}

// SampleFormat returns an abbreviation of the stream's sample format ("s16",
// "flt", "u8p", etc.). It can be assigned to SinkConfig.SampleFormat.
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
