package aurelib

/*
#cgo pkg-config: libavformat libavcodec

#include <libavformat/avformat.h>
#include <libavcodec/avcodec.h>
*/
import "C"
import "fmt"
import "time"

func init() {
	C.av_register_all()
	SetLogLevel(LogPanic)
}

type LogLevel int

const (
	LogQuiet LogLevel = iota
	LogPanic
	LogFatal
	LogError
	LogWarning
	LogInfo
	LogVerbose
	LogDebug
	LogTrace
)

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

func NetworkInit() {
	C.avformat_network_init()
}

func NetworkDeinit() {
	C.avformat_network_deinit()
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

type StreamInfo struct {
	SampleRate uint

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

func (info *StreamInfo) ChannelLayout() string {
	return channelLayoutToString(info.channelLayout)
}

func sampleFormatToString(sampleFormat int32) string {
	return C.GoString(C.av_get_sample_fmt_name(C.enum_AVSampleFormat(sampleFormat)))
}

func (info *StreamInfo) SampleFormat() string {
	return sampleFormatToString(info.sampleFormat)
}

type Frame struct {
	frame *C.AVFrame
	Size  uint
}

func (frame Frame) Destroy() {
	if frame.frame != nil {
		C.av_frame_free(&frame.frame)
	}
	frame.Size = 0
}

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
