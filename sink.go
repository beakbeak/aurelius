package main

/*
#cgo pkg-config: libavformat libavcodec libavutil

#include <libavformat/avformat.h>
#include <stdlib.h>

static int avErrorEOF() {
    return AVERROR_EOF;
}

static int avErrorEAGAIN() {
    return AVERROR(EAGAIN);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type AudioSinkOptions struct {
	Channels         uint    // default 2
	ChannelLayout    string  // optional; overrides Channels
	SampleRate       uint    // default 44100
	SampleFormat     string  // default is codec-dependent
	Codec            string  // default "flac"
	CompressionLevel int     // used for flac
	Quality          float32 // used for libmp3lame, libvorbis
	BitRate          uint
}

func NewAudioSinkOptions() *AudioSinkOptions {
	return &AudioSinkOptions{
		Channels:         2,
		SampleRate:       44100,
		Codec:            "flac",
		CompressionLevel: -1,
		Quality:          -2,
	}
}

func (options *AudioSinkOptions) getChannels() (C.int, C.uint64_t) {
	var channels C.int
	var channelLayout C.uint64_t

	if options.ChannelLayout != "" {
		channelLayoutName := C.CString(options.ChannelLayout)
		defer C.free(unsafe.Pointer(channelLayoutName))

		if channelLayout = C.av_get_channel_layout(channelLayoutName); channelLayout != 0 {
			channels = C.av_get_channel_layout_nb_channels(channelLayout)
		}
	}
	if channels == 0 {
		channels = C.int(options.Channels)
		channelLayout = C.uint64_t(C.av_get_default_channel_layout(channels))
	}
	return channels, channelLayout
}

func (options *AudioSinkOptions) getSampleFormat(defaultFormat int32) int32 {
	if options.SampleFormat != "" {
		formatName := C.CString(options.SampleFormat)
		defer C.free(unsafe.Pointer(formatName))

		return C.av_get_sample_fmt(formatName)
	}
	return defaultFormat
}

func (options *AudioSinkOptions) getCodec() *C.struct_AVCodec {
	cCodecName := C.CString(options.Codec)
	defer C.free(unsafe.Pointer(cCodecName))

	return C.avcodec_find_encoder_by_name(cCodecName)
}

type AudioSink struct {
	ioCtx     *C.struct_AVIOContext
	formatCtx *C.struct_AVFormatContext
	codecCtx  *C.struct_AVCodecContext

	runningTime C.int64_t
	frame       *C.struct_AVFrame
}

func (sink *AudioSink) Destroy() {
	if sink.frame != nil {
		C.av_frame_free(&sink.frame)
	}
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
	options *AudioSinkOptions,
) (*AudioSink, error) {
	success := false
	sink := AudioSink{}
	defer func() {
		if !success {
			sink.Destroy()
		}
	}()

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// open the output file for writing
	if avErr := C.avio_open(&sink.ioCtx, cPath, C.AVIO_FLAG_WRITE); avErr < 0 {
		return nil, fmt.Errorf("failed to open file: %v", avErr2Str(avErr))
	}

	// create a new format context for the output container format
	if sink.formatCtx = C.avformat_alloc_context(); sink.formatCtx == nil {
		return nil, fmt.Errorf("failed to allocate format context")
	}

	// guess the desired container format based on the file extension
	if sink.formatCtx.oformat = C.av_guess_format(nil, cPath, nil); sink.formatCtx.oformat == nil {
		return nil, fmt.Errorf("failed to determine output file format")
	}

	// associate the output file with the container format context
	sink.formatCtx.pb = sink.ioCtx

	// create a new audio stream in the output file container
	var stream *C.struct_AVStream
	if stream = C.avformat_new_stream(sink.formatCtx, nil); stream == nil {
		return nil, fmt.Errorf("failed to create output stream")
	}

	// set the sample rate for the container
	stream.time_base.num = 1
	stream.time_base.den = C.int(options.SampleRate)

	var codec *C.struct_AVCodec
	if codec = options.getCodec(); codec == nil {
		return nil, fmt.Errorf("failed to find output encoder '%v'", options.Codec)
	}

	if sink.codecCtx = C.avcodec_alloc_context3(codec); sink.codecCtx == nil {
		return nil, fmt.Errorf("failed to allocate encoding context")
	}

	sink.codecCtx.channels, sink.codecCtx.channel_layout = options.getChannels()

	sink.codecCtx.sample_rate = C.int(options.SampleRate)
	sink.codecCtx.sample_fmt = options.getSampleFormat(*codec.sample_fmts)
	sink.codecCtx.time_base = stream.time_base

	// some container formats (like MP4) require global headers to be present.
	// mark the encoder so that it behaves accordingly
	if (sink.formatCtx.oformat.flags & C.AVFMT_GLOBALHEADER) != 0 {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_GLOBAL_HEADER
	}

	if options.CompressionLevel >= 0 {
		sink.codecCtx.compression_level = C.int(options.CompressionLevel)
	} else if options.Quality >= -1. {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_QSCALE
		sink.codecCtx.global_quality = C.int(options.Quality * C.FF_QP2LAMBDA)
	} else if options.BitRate != 0 {
		sink.codecCtx.bit_rate = C.int64_t(options.BitRate)
	}

	// open the encoder for the audio stream to use it later
	if avErr := C.avcodec_open2(sink.codecCtx, codec, nil); avErr < 0 {
		return nil, fmt.Errorf("failed to open output codec: %v", avErr2Str(avErr))
	}

	if avErr := C.avcodec_parameters_from_context(stream.codecpar, sink.codecCtx); avErr < 0 {
		return nil, fmt.Errorf("failed to initialize stream parameters")
	}

	if avErr := C.avformat_write_header(sink.formatCtx, nil); avErr < 0 {
		return nil, fmt.Errorf("failed to write header: %v", avErr2Str(avErr))
	}

	success = true
	return &sink, nil
}

func (sink *AudioSink) FrameSize() int {
	value := sink.codecCtx.frame_size
	if value <= 0 {
		return 4096
	}
	return int(value)
}

func (sink *AudioSink) encodeFrames(fifo *AudioFIFO) error {
	frameSize := sink.FrameSize()
	if fifo.Size() < frameSize {
		frameSize = fifo.Size()
	}

	if sink.frame == nil {
		if sink.frame = C.av_frame_alloc(); sink.frame == nil {
			return fmt.Errorf("failed to allocate input frame")
		}
	}
	defer C.av_frame_unref(sink.frame)

	sink.frame.nb_samples = C.int(frameSize)
	sink.frame.channel_layout = sink.codecCtx.channel_layout
	sink.frame.format = C.int(sink.codecCtx.sample_fmt)
	sink.frame.sample_rate = sink.codecCtx.sample_rate

	if err := C.av_frame_get_buffer(sink.frame, 0); err < 0 {
		return fmt.Errorf("failed to allocate output frame buffer: %s", avErr2Str(err))
	}

	if fifo.read(
		(*unsafe.Pointer)(unsafe.Pointer(&sink.frame.data[0])), C.int(frameSize),
	) < C.int(frameSize) {
		return fmt.Errorf("failed to read from FIFO")
	}

	// XXX
	sink.frame.pts = sink.runningTime
	sink.runningTime += C.int64_t(frameSize)

	if err := C.avcodec_send_frame(sink.codecCtx, sink.frame); err < 0 {
		return fmt.Errorf("failed to encode frame: %s", avErr2Str(err))
	}

	if eof, err := sink.writeFrames(); eof {
		return fmt.Errorf("unexpected EOF from encoder")
	} else if err != nil {
		return err
	}
	return nil
}

func (sink *AudioSink) flush(fifo *AudioFIFO) error {
	for fifo.Size() > 0 {
		if err := sink.encodeFrames(fifo); err != nil {
			return err
		}
	}
	if err := C.avcodec_send_frame(sink.codecCtx, nil); err < 0 {
		return fmt.Errorf("failed to encode NULL frame: %s", avErr2Str(err))
	}
	_, err := sink.writeFrames()
	return err
}

func (sink *AudioSink) writeFrames() (bool /*eof*/, error) {
	var packet C.AVPacket
	packet.init()
	defer C.av_packet_unref(&packet)

	for {
		// calls av_packet_unref()
		if err := C.avcodec_receive_packet(sink.codecCtx, &packet); err == C.avErrorEAGAIN() {
			break
		} else if err == C.avErrorEOF() {
			return true, nil
		} else if err < 0 {
			return false, fmt.Errorf("failed to receive packet from encoder: %v", avErr2Str(err))
		}

		if err := C.av_write_frame(sink.formatCtx, &packet); err < 0 {
			return false, fmt.Errorf("failed to write frame: %s", avErr2Str(err))
		}
	}
	return false, nil
}

func (sink *AudioSink) writeTrailer() error {
	if err := C.av_write_trailer(sink.formatCtx); err < 0 {
		return fmt.Errorf("failed to write trailer: %s", avErr2Str(err))
	}
	return nil
}
