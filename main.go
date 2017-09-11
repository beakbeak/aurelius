package main

/*
#cgo pkg-config: libavformat libavcodec libavutil

#include <libavformat/avformat.h>
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"os"
	"unsafe"
)

func main() {
	C.av_register_all()
	C.avformat_network_init()
	defer C.avformat_network_deinit()

	var src *AudioFileSource
	var err error
	if src, err = decode(os.Args[1]); err != nil {
		fmt.Println(err)
	}

	src.dumpFormat()
}

func avErr2Str(code C.int) string {
	var buffer [C.AV_ERROR_MAX_STRING_SIZE]C.char
	if C.av_strerror(code, &buffer[0], C.AV_ERROR_MAX_STRING_SIZE) < 0 {
		return "Unknown error"
	}
	return C.GoString(&buffer[0])
}

type AudioFileSource struct {
	Path string

	formatCtx *C.struct_AVFormatContext
	codecCtx  *C.struct_AVCodecContext
}

func (s *AudioFileSource) Destroy() {
	if s.codecCtx != nil {
		C.avcodec_free_context(&s.codecCtx)
	}
	if s.formatCtx != nil {
		C.avformat_close_input(&s.formatCtx)
	}
}

func (s *AudioFileSource) dumpFormat() {
	cPath := C.CString(s.Path)
	defer C.free(unsafe.Pointer(cPath))
	C.av_dump_format(s.formatCtx, 0, cPath, 0)
}

func decode(path string) (*AudioFileSource, error) {
	success := false

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// open file
	var formatCtx *C.struct_AVFormatContext
	if err := C.avformat_open_input(&formatCtx, cPath, nil, nil); err < 0 {
		return nil, fmt.Errorf("Failed to open file: %v", avErr2Str(err))
	}
	defer func() {
		if !success {
			C.avformat_close_input(&formatCtx)
		}
	}()

	// gather streams
	if err := C.avformat_find_stream_info(formatCtx, nil); err < 0 {
		return nil, fmt.Errorf("Failed to find stream info: %v", avErr2Str(err))
	}

	// find first audio stream
	var audioStream *C.struct_AVStream
	{
		streamCount := formatCtx.nb_streams
		streams := (*[1 << 30]*C.struct_AVStream)(unsafe.Pointer(formatCtx.streams))[:streamCount:streamCount]
		for i := C.uint(0); i < streamCount; i++ {
			if streams[i].codecpar.codec_type == C.AVMEDIA_TYPE_AUDIO {
				audioStream = streams[i]
				break
			}
		}
	}

	if audioStream == nil {
		return nil, errors.New("No audio streams")
	}

	codec := C.avcodec_find_decoder(audioStream.codecpar.codec_id)
	if codec == nil {
		return nil, errors.New("Failed to find decoder")
	}

	codecCtx := C.avcodec_alloc_context3(codec)
	if codecCtx == nil {
		return nil, errors.New("Failed to allocate decoding context")
	}
	defer func() {
		if !success {
			C.avcodec_free_context(&codecCtx)
		}
	}()

	if err := C.avcodec_parameters_to_context(codecCtx, audioStream.codecpar); err < 0 {
		return nil, fmt.Errorf("Failed to copy codec parameters: %v", avErr2Str(err))
	}

	if err := C.avcodec_open2(codecCtx, codec, nil); err < 0 {
		return nil, fmt.Errorf("Failed to open decoder: %v", avErr2Str(err))
	}

	success = true
	return &AudioFileSource{path, formatCtx, codecCtx}, nil
}
