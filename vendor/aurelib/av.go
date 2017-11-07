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
