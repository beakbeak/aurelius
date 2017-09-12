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
	if src, err = newAudioFileSource(os.Args[1]); err != nil {
		fmt.Println(err)
	}
	defer src.Destroy()

	src.dumpFormat()

	var dest *AudioFileSink
	if dest, err = newAudioFileSink(os.Args[2], src); err != nil {
		fmt.Println(err)
	}
	defer dest.Destroy()
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

func (src *AudioFileSource) Destroy() {
	if src.codecCtx != nil {
		C.avcodec_free_context(&src.codecCtx)
	}
	if src.formatCtx != nil {
		C.avformat_close_input(&src.formatCtx)
	}
}

func (src *AudioFileSource) dumpFormat() {
	cPath := C.CString(src.Path)
	defer C.free(unsafe.Pointer(cPath))
	C.av_dump_format(src.formatCtx, 0, cPath, 0)
}

func newAudioFileSource(path string) (*AudioFileSource, error) {
	success := false

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// open file
	var formatCtx *C.struct_AVFormatContext
	if err := C.avformat_open_input(&formatCtx, cPath, nil, nil); err < 0 {
		return nil, fmt.Errorf("failed to open file: %v", avErr2Str(err))
	}
	defer func() {
		if !success {
			C.avformat_close_input(&formatCtx)
		}
	}()

	// gather streams
	if err := C.avformat_find_stream_info(formatCtx, nil); err < 0 {
		return nil, fmt.Errorf("failed to find stream info: %v", avErr2Str(err))
	}

	// find first audio stream
	var audioStream *C.struct_AVStream
	{
		streamCount := formatCtx.nb_streams
		streams := (*[1 << 30]*C.struct_AVStream)(unsafe.Pointer(formatCtx.streams))[:streamCount:streamCount]
		for _, stream := range streams {
			if stream.codecpar.codec_type == C.AVMEDIA_TYPE_AUDIO {
				audioStream = stream
				break
			}
		}
	}

	if audioStream == nil {
		return nil, errors.New("no audio streams")
	}

	codec := C.avcodec_find_decoder(audioStream.codecpar.codec_id)
	if codec == nil {
		return nil, errors.New("failed to find decoder")
	}

	codecCtx := C.avcodec_alloc_context3(codec)
	if codecCtx == nil {
		return nil, errors.New("failed to allocate decoding context")
	}
	defer func() {
		if !success {
			C.avcodec_free_context(&codecCtx)
		}
	}()

	if err := C.avcodec_parameters_to_context(codecCtx, audioStream.codecpar); err < 0 {
		return nil, fmt.Errorf("failed to copy codec parameters: %v", avErr2Str(err))
	}

	if err := C.avcodec_open2(codecCtx, codec, nil); err < 0 {
		return nil, fmt.Errorf("failed to open decoder: %v", avErr2Str(err))
	}

	success = true
	return &AudioFileSource{path, formatCtx, codecCtx}, nil
}

type AudioFileSink struct {
	ioCtx     *C.struct_AVIOContext
	formatCtx *C.struct_AVFormatContext
	codecCtx  *C.struct_AVCodecContext
}

func (sink *AudioFileSink) Destroy() {
	if sink.codecCtx != nil {
		C.avcodec_free_context(&sink.codecCtx)
	}
	if sink.formatCtx != nil {
		C.avformat_free_context(sink.formatCtx)
		sink.formatCtx = nil
	}
	if sink.ioCtx != nil {
		C.avio_closep(&sink.ioCtx)
	}
}

func newAudioFileSink(
	path string,
	src *AudioFileSource,
) (*AudioFileSink, error) {
	success := false

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// open the output file for writing
	var ioCtx *C.struct_AVIOContext
	if err := C.avio_open(&ioCtx, cPath, C.AVIO_FLAG_WRITE); err < 0 {
		return nil, fmt.Errorf("failed to open file: %v", avErr2Str(err))
	}
	defer func() {
		if !success {
			C.avio_closep(&ioCtx)
		}
	}()

	// create a new format context for the output container format
	var formatCtx *C.struct_AVFormatContext
	if formatCtx = C.avformat_alloc_context(); formatCtx == nil {
		return nil, fmt.Errorf("failed to allocate format context")
	}
	defer func() {
		if !success {
			C.avformat_free_context(formatCtx)
		}
	}()

	// guess the desired container format based on the file extension
	if formatCtx.oformat = C.av_guess_format(nil, cPath, nil); formatCtx.oformat == nil {
		return nil, fmt.Errorf("failed to determine output file format")
	}

	// associate the output file with the container format context
	formatCtx.pb = ioCtx

	// create a new audio stream in the output file container
	var stream *C.struct_AVStream
	if stream = C.avformat_new_stream(formatCtx, nil); stream == nil {
		return nil, fmt.Errorf("failed to create output stream")
	}

	// set the sample rate for the container
	stream.time_base.den = src.codecCtx.sample_rate
	stream.time_base.num = 1

	codec := C.avcodec_find_encoder(C.AV_CODEC_ID_FLAC)
	if codec == nil {
		return nil, fmt.Errorf("failed to find output encoder")
	}

	codecCtx := C.avcodec_alloc_context3(codec)
	if codecCtx == nil {
		return nil, fmt.Errorf("failed to allocate encoding context")
	}
	defer func() {
		if !success {
			C.avcodec_free_context(&codecCtx)
		}
	}()

	codecCtx.channels = src.codecCtx.channels
	codecCtx.channel_layout = src.codecCtx.channel_layout // C.av_get_default_channel_layout(codecCtx.channels)
	codecCtx.sample_rate = src.codecCtx.sample_rate
	codecCtx.sample_fmt = *codec.sample_fmts // ?
	// codecCtx.bit_rate

	// some container formats (like MP4) require global headers to be present.
	// mark the encoder so that it behaves accordingly
	if (formatCtx.oformat.flags & C.AVFMT_GLOBALHEADER) != 0 {
		codecCtx.flags |= C.AV_CODEC_FLAG_GLOBAL_HEADER
	}

	// open the encoder for the audio stream to use it later
	if err := C.avcodec_open2(codecCtx, codec, nil); err < 0 {
		return nil, fmt.Errorf("failed to open output codec: %v", avErr2Str(err))
	}

	if err := C.avcodec_parameters_from_context(stream.codecpar, codecCtx); err < 0 {
		return nil, fmt.Errorf("failed to initialize stream parameters")
	}

	success = true
	return &AudioFileSink{ioCtx, formatCtx, codecCtx}, nil
}
