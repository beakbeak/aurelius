package aurelib

/*
#cgo pkg-config: libavformat libavcodec libavutil

#include <libavformat/avformat.h>
#include <stdlib.h>

static int
avErrorEOF() {
	return AVERROR_EOF;
}

static int
avErrorEAGAIN() {
	return AVERROR(EAGAIN);
}

// A growable data buffer.
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

// Append data to the Buffer stored in opaque, growing it if necessary.
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

// Consume data_size bytes from the beginning of the Buffer stored in opaque.
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

// A Sink encodes raw audio data.
type Sink interface {
	// Destroy frees any resources held by the Sink so that it may be discarded.
	Destroy()

	// StreamInfo returns an object describing the format of audio data accepted
	// by the Sink.
	StreamInfo() StreamInfo

	// FrameSize returns the number of samples per Frame expected by Encode.
	FrameSize() uint

	// Encode encodes a chunk of audio data. It takes ownership of the Frame, so
	// the caller should not call Frame.Destroy after calling Encode.
	//
	// Passing an empty Frame will cause the encoder to flush any buffered data
	// and conclude encoding. See FlushSink.
	//
	// The return value will be true when the encoder has completed encoding and
	// will no longer accept any Frames.
	Encode(frame Frame) (done bool, err error)

	// WriteTrailer finalizes data written to the audio container format.
	//
	// It should be called after flushing the encoder by passing an empty Frame
	// to Encode. See FlushSink.
	WriteTrailer() error
}

// SinkConfig contains audio format and encoding configuration used by
// NewBufferSink and NewFileSink. It should be created with NewSinkConfig to
// provide default values.
type SinkConfig struct {
	Channels uint // The number of channels. (Default: 2)

	// ChannelLayout describes the number and position of audio channels. It
	// uses the same format as FFmpeg's av_get_channel_layout().
	//
	// ChannelLayout overrides Channels if specified.
	//
	// From the FFmpeg documentation:
	//     ... can be one or several of the following notations, separated by '+' or '|':
	//
	//     * the name of an usual channel layout (mono, stereo, 4.0, quad, 5.0,
	//       5.0(side), 5.1, 5.1(side), 7.1, 7.1(wide), downmix);
	//     * the name of a single channel (FL, FR, FC, LFE, BL, BR, FLC, FRC,
	//       BC, SL, SR, TC, TFL, TFC, TFR, TBL, TBC, TBR, DL, DR);
	//     * a number of channels, in decimal, followed by 'c', yielding the
	//       default channel layout for that number of channels
	//     * a channel layout mask, in hexadecimal starting with "0x" (see the
	//       AV_CH_* macros).
	//
	//     Example: "stereo+FC" = "2c+FC" = "2c+1c" = "0x7"
	ChannelLayout string

	SampleRate uint // Sample rate in Hz. (Default: 44100)

	// SampleFormat is an abbreviation of the stream's sample format ("s16",
	// "flt", "u8p", etc.). If unspecified, the format is determined by the
	// audio codec.
	//
	// See FFmpeg's documentation for AVSampleFormat for a full list of formats.
	SampleFormat string

	// Codec indicates the audio encoder to use ("flac", "libmp3lame",
	// "libvorbis", etc.). It is passed to FFmpeg's
	// avcodec_find_encoder_by_name(). (Default: "pcm_s16le")
	Codec string

	// CompressionLevel controls the strength of compression in a
	// codec-dependent way, and corresponds to FFmpeg's
	// AVCodecContext.compression_level, used by FLAC encoding. A negative value
	// is ignored. (Default: -1)
	//
	// CompressionLevel overrides Quality if specified.
	CompressionLevel int

	// Quality controls encoding quality in a codec-dependent way, and
	// corresponds to FFmpeg's AVCodecContext.global_quality, used by, e.g., the
	// libmp3lame and libvorbis encoders. A value less than -1 is ignored.
	// (Default: -2)
	//
	// Quality overrides BitRate if specified.
	Quality float32

	// BitRate sets a target number of bits/s to be produced by audio encoding,
	// and corresponds to FFmpeg's AVCodecContext.bit_rate. A value of 0 is
	// ignored. (Default: 0)
	BitRate uint

	// BitExact controls whether to avoid randomness in encoding and muxing. It
	// corresponds to FFmpeg's AVFMT_FLAG_BITEXACT and AV_CODEC_FLAG_BITEXACT.
	// (Default: false)
	//
	// This should be enabled when deterministic output is needed, such as when
	// performing automated testing.
	BitExact bool
}

// NewSinkConfig creates a new SinkConfig object with default values.
func NewSinkConfig() *SinkConfig {
	return &SinkConfig{
		Channels:         2,
		SampleRate:       44100,
		Codec:            "pcm_s16le",
		CompressionLevel: -1,
		Quality:          -2,
	}
}

func (config *SinkConfig) getChannels() (C.int, C.uint64_t) {
	var channels C.int
	var channelLayout C.uint64_t

	if config.ChannelLayout != "" {
		channelLayoutName := C.CString(config.ChannelLayout)
		defer C.free(unsafe.Pointer(channelLayoutName))

		if channelLayout = C.av_get_channel_layout(channelLayoutName); channelLayout != 0 {
			channels = C.av_get_channel_layout_nb_channels(channelLayout)
		}
	}
	if channels == 0 {
		channels = C.int(config.Channels)
		channelLayout = C.uint64_t(C.av_get_default_channel_layout(channels))
	}
	return channels, channelLayout
}

func (codec *C.AVCodec) allowedFormats() []C.enum_AVSampleFormat {
	if codec.sample_fmts == nil {
		return []C.enum_AVSampleFormat{C.AV_SAMPLE_FMT_NONE}
	}

	formatArray := (*[1 << 30]C.enum_AVSampleFormat)(unsafe.Pointer(codec.sample_fmts))
	formatCount := 0
	for formatArray[formatCount] != -1 {
		formatCount++
	}

	return formatArray[:formatCount:formatCount]
}

func (config *SinkConfig) getSampleFormat(
	allowedFormats []C.enum_AVSampleFormat,
) C.enum_AVSampleFormat {
	if config.SampleFormat == "" {
		return allowedFormats[0]
	}

	findFormat := func(format C.enum_AVSampleFormat) bool {
		for _, allowedFormat := range allowedFormats {
			if allowedFormat == format {
				return true
			}
		}
		return false
	}

	formatName := C.CString(config.SampleFormat)
	defer C.free(unsafe.Pointer(formatName))

	format := C.av_get_sample_fmt(formatName)

	if findFormat(format) {
		return format
	}

	if C.av_sample_fmt_is_planar(format) != 0 {
		format = C.av_get_packed_sample_fmt(format)
	} else {
		format = C.av_get_planar_sample_fmt(format)
	}

	if findFormat(format) {
		return format
	}

	return allowedFormats[0]
}

func (config *SinkConfig) getCodec() *C.AVCodec {
	cCodecName := C.CString(config.Codec)
	defer C.free(unsafe.Pointer(cCodecName))

	return C.avcodec_find_encoder_by_name(cCodecName)
}

type sinkBase struct {
	formatCtx *C.AVFormatContext
	codecCtx  *C.AVCodecContext

	runningTime C.int64_t
}

// Destroy frees any resources held by the Sink so that it may be discarded.
func (sink *sinkBase) Destroy() {
	if sink.codecCtx != nil {
		C.avcodec_free_context(&sink.codecCtx)
	}
	if sink.formatCtx != nil {
		C.avformat_free_context(sink.formatCtx)
		sink.formatCtx = nil
	}
}

// A FileSink writes encoded audio to a file in the local filesystem.
type FileSink struct {
	sinkBase
	ioCtx *C.AVIOContext
}

// Destroy frees any resources held by the Sink so that it may be discarded.
func (sink *FileSink) Destroy() {
	sink.sinkBase.Destroy()

	if sink.ioCtx != nil {
		C.avio_closep(&sink.ioCtx)
	}
}

// NewFileSink creates a new FileSink that writes encoded audio to the file
// specified by path. The container format is inferred from the file extension.
//
// The Sink is backed by a heap-allocated C data structure, so it must be
// destroyed with Destroy before it is discarded.
func NewFileSink(
	path string,
	config *SinkConfig,
) (*FileSink, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// guess the desired container format based on the file extension
	format := C.av_guess_format(nil, cPath, nil)
	if format == nil {
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

	if err := sink.init(format, sink.ioCtx, config); err != nil {
		return nil, err
	}

	success = true
	return &sink, nil
}

// A BufferSink writes encoded audio to a memory buffer.
type BufferSink struct {
	sinkBase
	ioCtx  *C.AVIOContext
	buffer *C.Buffer
}

// Destroy frees any resources held by the Sink so that it may be discarded.
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

// NewBufferSink creates a new BufferSink that will store audio data in the
// container format specified by containerFormatName (e.g., "matroska", "mp3",
// "ogg", "flac" -- different from the codec specified by SinkConfig.Codec).
// The format name is interpreted by FFmpeg's av_guess_format().
//
// The Sink is backed by a heap-allocated C data structure, so it must be
// destroyed with Destroy before it is discarded.
func NewBufferSink(
	containerFormatName string,
	config *SinkConfig,
) (*BufferSink, error) {
	cFormatName := C.CString(containerFormatName)
	defer C.free(unsafe.Pointer(cFormatName))

	format := C.av_guess_format(cFormatName, nil, nil)
	if format == nil {
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

	if err := sink.init(format, sink.ioCtx, config); err != nil {
		return nil, err
	}

	success = true
	return &sink, nil
}

// Buffer returns the current contents of the BufferSink's encoded data buffer.
func (sink *BufferSink) Buffer() []byte {
	return (*[1 << 30]byte)(unsafe.Pointer(sink.buffer.data))[:sink.buffer.size:sink.buffer.capacity]
}

// Drain discards at most the first byteCount bytes from the BufferSink's
// encoded data buffer. It returns the number of bytes discarded.
func (sink *BufferSink) Drain(byteCount uint) uint {
	return uint(C.Buffer_read(unsafe.Pointer(sink.buffer), nil, C.int(byteCount)))
}

func (sink *sinkBase) init(
	format *C.AVOutputFormat,
	ioCtx *C.AVIOContext,
	config *SinkConfig,
) error {
	if sink.formatCtx = C.avformat_alloc_context(); sink.formatCtx == nil {
		return fmt.Errorf("failed to allocate format context")
	}

	sink.formatCtx.oformat = format
	sink.formatCtx.pb = ioCtx

	if config.BitExact {
		sink.formatCtx.flags |= C.AVFMT_FLAG_BITEXACT
	}

	stream := C.avformat_new_stream(sink.formatCtx, nil)
	if stream == nil {
		return fmt.Errorf("failed to create output stream")
	}

	// set the sample rate for the container
	stream.time_base.num = 1
	stream.time_base.den = C.int(config.SampleRate)

	codec := config.getCodec()
	if codec == nil {
		return fmt.Errorf("failed to find output encoder '%v'", config.Codec)
	}

	if sink.codecCtx = C.avcodec_alloc_context3(codec); sink.codecCtx == nil {
		return fmt.Errorf("failed to allocate encoding context")
	}

	sink.codecCtx.channels, sink.codecCtx.channel_layout = config.getChannels()
	sink.codecCtx.sample_rate = C.int(config.SampleRate)
	sink.codecCtx.sample_fmt = config.getSampleFormat(codec.allowedFormats())
	sink.codecCtx.time_base = stream.time_base

	if config.CompressionLevel >= 0 {
		sink.codecCtx.compression_level = C.int(config.CompressionLevel)
	} else if config.Quality >= -1. {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_QSCALE
		sink.codecCtx.global_quality = C.int(config.Quality * C.FF_QP2LAMBDA)
	} else if config.BitRate != 0 {
		sink.codecCtx.bit_rate = C.int64_t(config.BitRate)
	}

	// some container formats (like MP4) require global headers to be present.
	// mark the encoder so that it behaves accordingly
	if (sink.formatCtx.oformat.flags & C.AVFMT_GLOBALHEADER) != 0 {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_GLOBAL_HEADER
	}

	if config.BitExact {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_BITEXACT
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

// FrameSize returns the number of samples per Frame expected by Encode.
func (sink *sinkBase) FrameSize() uint {
	value := sink.codecCtx.frame_size
	if value <= 0 {
		return 4096
	}
	return uint(value)
}

// Encode encodes a chunk of audio data. It takes ownership of the Frame, so the
// caller should not call Frame.Destroy after calling Encode.
//
// Passing an empty Frame will cause the encoder to flush any buffered data and
// conclude encoding. See FlushSink.
//
// The return value will be true when the encoder has completed encoding and
// will no longer accept any Frames.
func (sink *sinkBase) Encode(frame Frame) (done bool, err error) {
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

func (sink *sinkBase) write() (eof bool, _ error) {
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

// WriteTrailer finalizes data written to the audio container format.
//
// It should be called after flushing the encoder by passing an empty Frame to
// Encode. See FlushSink.
func (sink *sinkBase) WriteTrailer() error {
	if err := C.av_write_trailer(sink.formatCtx); err < 0 {
		return fmt.Errorf("failed to write trailer: %s", avErr2Str(err))
	}
	return nil
}

// StreamInfo returns an object describing the format of audio data accepted by
// the Sink.
func (sink *sinkBase) StreamInfo() StreamInfo {
	return sink.codecCtx.streamInfo()
}

// FlushSink flushes a Sink by passing an empty Frame to Encode and calling
// WriteTrailer.
func FlushSink(sink Sink) error {
	if _, err := sink.Encode(Frame{}); err != nil {
		return fmt.Errorf("failed to encode empty frame: %v", err)
	}
	return sink.WriteTrailer()
}
