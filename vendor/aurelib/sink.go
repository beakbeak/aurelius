package aurelib

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

typedef struct Buffer {
	size_t size;
	size_t capacity;
	uint8_t* data;
} Buffer;

static Buffer*
Buffer_new() {
	Buffer* b;

	b = (Buffer*)malloc(sizeof(Buffer));
	b->size = b->capacity = 0;
	b->data = NULL;
	return b;
}

static void
Buffer_delete(Buffer* b) {
	if (b) {
		free(b->data);
	}
	free(b);
}

typedef int (*Buffer_write_t)(void*, uint8_t*, int);

int
Buffer_write(
	void* opaque,
	uint8_t* data,
	int data_size)
{
	Buffer* buffer;
	size_t offset;

	buffer = (Buffer*)opaque;
	offset = buffer->size;
	buffer->size += data_size;
	if (buffer->size > buffer->capacity) {
		buffer->data = (uint8_t*)realloc(buffer->data, buffer->size);
		buffer->capacity = buffer->size;
	}
	memcpy(buffer->data + offset, data, data_size);
	return data_size;
}

typedef int (*Buffer_read_t)(void*, uint8_t*, int);

int
Buffer_read(
	void* opaque,
	uint8_t* data,
	int data_size)
{
	Buffer* buffer;

	buffer = (Buffer*)opaque;
	if ((int)buffer->size < data_size) {
		data_size = (int)buffer->size;
	}
	if (data_size <= 0) {
		return 0;
	}

	if (data) {
		memcpy(data, buffer->data, data_size);
	}
	if ((size_t)data_size < buffer->size) {
		memmove(buffer->data, buffer->data + data_size, buffer->size - data_size);
	}
	buffer->size -= data_size;
	return data_size;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type SinkOptions struct {
	Channels         uint    // default 2
	ChannelLayout    string  // optional; overrides Channels
	SampleRate       uint    // default 44100
	SampleFormat     string  // default is codec-dependent
	Codec            string  // default "flac"
	CompressionLevel int     // used for flac
	Quality          float32 // used for libmp3lame, libvorbis
	BitRate          uint
}

type Sink interface {
	Destroy()
	FrameSize() int
	SampleRate() int
	Encode(frame Frame) (bool /*done*/, error)
	WriteTrailer() error

	codecContext() *C.struct_AVCodecContext
}

func NewSinkOptions() *SinkOptions {
	return &SinkOptions{
		Channels:         2,
		SampleRate:       44100,
		Codec:            "flac",
		CompressionLevel: -1,
		Quality:          -2,
	}
}

func (options *SinkOptions) getChannels() (C.int, C.uint64_t) {
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

func (options *SinkOptions) getSampleFormat(defaultFormat int32) int32 {
	if options.SampleFormat != "" {
		formatName := C.CString(options.SampleFormat)
		defer C.free(unsafe.Pointer(formatName))

		return C.av_get_sample_fmt(formatName)
	}
	return defaultFormat
}

func (options *SinkOptions) getCodec() *C.struct_AVCodec {
	cCodecName := C.CString(options.Codec)
	defer C.free(unsafe.Pointer(cCodecName))

	return C.avcodec_find_encoder_by_name(cCodecName)
}

type sinkBase struct {
	formatCtx *C.struct_AVFormatContext
	codecCtx  *C.struct_AVCodecContext

	runningTime C.int64_t
}

func (sink *sinkBase) Destroy() {
	if sink.codecCtx != nil {
		C.avcodec_free_context(&sink.codecCtx)
	}
	if sink.formatCtx != nil {
		C.avformat_free_context(sink.formatCtx)
		sink.formatCtx = nil
	}
}

type FileSink struct {
	sinkBase
	ioCtx *C.struct_AVIOContext
}

func (sink *FileSink) Destroy() {
	sink.sinkBase.Destroy()

	if sink.ioCtx != nil {
		C.avio_closep(&sink.ioCtx)
	}
}

func NewFileSink(
	path string,
	options *SinkOptions,
) (*FileSink, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// guess the desired container format based on the file extension
	var format *C.struct_AVOutputFormat
	if format = C.av_guess_format(nil, cPath, nil); format == nil {
		return nil, fmt.Errorf("failed to determine output file format")
	}

	success := false
	sink := FileSink{}
	defer func() {
		if !success {
			sink.Destroy()
		}
	}()

	if avErr := C.avio_open(&sink.ioCtx, cPath, C.AVIO_FLAG_WRITE); avErr < 0 {
		return nil, fmt.Errorf("failed to open file: %v", avErr2Str(avErr))
	}

	if err := sink.init(format, sink.ioCtx, options); err != nil {
		return nil, err
	}

	success = true
	return &sink, nil
}

type BufferSink struct {
	sinkBase
	ioCtx  *C.struct_AVIOContext
	buffer *C.struct_Buffer
}

func (sink *BufferSink) Destroy() {
	sink.sinkBase.Destroy()

	if sink.ioCtx != nil {
		C.av_free(unsafe.Pointer(sink.ioCtx.buffer))
		C.av_free(unsafe.Pointer(sink.ioCtx))
		sink.ioCtx = nil
	}
	if sink.buffer != nil {
		C.Buffer_delete(sink.buffer)
		sink.buffer = nil
	}
}

func NewBufferSink(
	formatName string,
	options *SinkOptions,
) (*BufferSink, error) {
	cFormatName := C.CString(formatName)
	defer C.free(unsafe.Pointer(cFormatName))

	var format *C.struct_AVOutputFormat
	if format = C.av_guess_format(cFormatName, nil, nil); format == nil {
		return nil, fmt.Errorf("failed to determine container format")
	}

	success := false
	sink := BufferSink{}
	defer func() {
		if !success {
			sink.Destroy()
		}
	}()

	if sink.buffer = C.Buffer_new(); sink.buffer == nil {
		return nil, fmt.Errorf("failed to allocate buffer")
	}

	ioCtxBufferSize := 4096
	ioCtxBuffer := C.av_malloc(C.size_t(ioCtxBufferSize))

	if sink.ioCtx = C.avio_alloc_context(
		(*C.uchar)(ioCtxBuffer), C.int(ioCtxBufferSize), 1, unsafe.Pointer(sink.buffer),
		C.Buffer_read_t(unsafe.Pointer(C.Buffer_read)),
		C.Buffer_write_t(unsafe.Pointer(C.Buffer_write)), nil,
	); sink.ioCtx == nil {
		C.av_free(unsafe.Pointer(ioCtxBuffer))
		return nil, fmt.Errorf("failed to allocate I/O context")
	}

	if err := sink.init(format, sink.ioCtx, options); err != nil {
		return nil, err
	}

	success = true
	return &sink, nil
}

func (sink *BufferSink) Buffer() []byte {
	return (*[1 << 30]byte)(unsafe.Pointer(sink.buffer.data))[:sink.buffer.size:sink.buffer.capacity]
}

func (sink *BufferSink) Drain(byteCount uint) uint {
	return uint(C.Buffer_read(unsafe.Pointer(sink.buffer), nil, C.int(byteCount)))
}

func (sink *sinkBase) init(
	format *C.struct_AVOutputFormat,
	ioCtx *C.struct_AVIOContext,
	options *SinkOptions,
) error {
	if sink.formatCtx = C.avformat_alloc_context(); sink.formatCtx == nil {
		return fmt.Errorf("failed to allocate format context")
	}

	sink.formatCtx.oformat = format
	sink.formatCtx.pb = ioCtx

	var stream *C.struct_AVStream
	if stream = C.avformat_new_stream(sink.formatCtx, nil); stream == nil {
		return fmt.Errorf("failed to create output stream")
	}

	// set the sample rate for the container
	stream.time_base.num = 1
	stream.time_base.den = C.int(options.SampleRate)

	var codec *C.struct_AVCodec
	if codec = options.getCodec(); codec == nil {
		return fmt.Errorf("failed to find output encoder '%v'", options.Codec)
	}

	if sink.codecCtx = C.avcodec_alloc_context3(codec); sink.codecCtx == nil {
		return fmt.Errorf("failed to allocate encoding context")
	}

	sink.codecCtx.channels, sink.codecCtx.channel_layout = options.getChannels()
	sink.codecCtx.sample_rate = C.int(options.SampleRate)
	sink.codecCtx.sample_fmt = options.getSampleFormat(*codec.sample_fmts)
	sink.codecCtx.time_base = stream.time_base

	if options.CompressionLevel >= 0 {
		sink.codecCtx.compression_level = C.int(options.CompressionLevel)
	} else if options.Quality >= -1. {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_QSCALE
		sink.codecCtx.global_quality = C.int(options.Quality * C.FF_QP2LAMBDA)
	} else if options.BitRate != 0 {
		sink.codecCtx.bit_rate = C.int64_t(options.BitRate)
	}

	// some container formats (like MP4) require global headers to be present.
	// mark the encoder so that it behaves accordingly
	if (sink.formatCtx.oformat.flags & C.AVFMT_GLOBALHEADER) != 0 {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_GLOBAL_HEADER
	}

	if avErr := C.avcodec_open2(sink.codecCtx, codec, nil); avErr < 0 {
		return fmt.Errorf("failed to open output codec: %v", avErr2Str(avErr))
	}
	if avErr := C.avcodec_parameters_from_context(stream.codecpar, sink.codecCtx); avErr < 0 {
		return fmt.Errorf("failed to initialize stream parameters")
	}

	if avErr := C.avformat_write_header(sink.formatCtx, nil); avErr < 0 {
		return fmt.Errorf("failed to write header: %v", avErr2Str(avErr))
	}
	return nil
}

func (sink *sinkBase) codecContext() *C.struct_AVCodecContext {
	return sink.codecCtx
}

func (sink *sinkBase) FrameSize() int {
	value := sink.codecCtx.frame_size
	if value <= 0 {
		return 4096
	}
	return int(value)
}

func (sink *sinkBase) SampleRate() int {
	return int(sink.codecCtx.sample_rate)
}

// consumes Frame
func (sink *sinkBase) Encode(frame Frame) (bool /*done*/, error) {
	defer frame.Destroy()

	// XXX
	if !frame.IsEmpty() {
		frame.frame.pts = sink.runningTime
		sink.runningTime += C.int64_t(frame.Size)
	}

	if err := C.avcodec_send_frame(sink.codecCtx, frame.frame); err < 0 {
		return false, fmt.Errorf("%s", avErr2Str(err))
	}

	if eof, err := sink.write(); eof {
		if !frame.IsEmpty() {
			return true, fmt.Errorf("unexpected EOF from encoder")
		}
		return true, nil
	} else if err != nil {
		return false, err
	}
	return frame.IsEmpty(), nil
}

func (sink *sinkBase) write() (bool /*eof*/, error) {
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
			return false, fmt.Errorf("failed to receive packet from encoder: %s", avErr2Str(err))
		}

		if err := C.av_write_frame(sink.formatCtx, &packet); err < 0 {
			return false, fmt.Errorf("failed to write frame: %s", avErr2Str(err))
		}
	}
	return false, nil
}

func (sink *sinkBase) WriteTrailer() error {
	if err := C.av_write_trailer(sink.formatCtx); err < 0 {
		return fmt.Errorf("failed to write trailer: %s", avErr2Str(err))
	}
	return nil
}
