package aurelib

/*
#cgo pkg-config: libavformat libavcodec

#include <libavformat/avformat.h>
#include <libavcodec/avcodec.h>
*/
import "C"

func Init() {
	C.av_register_all()
	C.avformat_network_init()
	defer C.avformat_network_deinit()
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

	// not exposed because they store FFMPEG enum values directly
	sampleFormat  int32
	channelLayout int64
}

func (ctx *C.struct_AVCodecContext) streamInfo() StreamInfo {
	return StreamInfo{
		SampleRate:    uint(ctx.sample_rate),
		sampleFormat:  ctx.sample_fmt,
		channelLayout: int64(ctx.channelLayout()),
	}
}

func (info *StreamInfo) channelCount() C.int {
	return C.av_get_channel_layout_nb_channels(C.uint64_t(info.channelLayout))
}

type Frame struct {
	frame *C.struct_AVFrame
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
