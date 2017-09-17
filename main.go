package main

// TODO:
// - need to flush resampler?
// - don't resample if format is the same
// - replaygain
// - seamless playback of multiple files
// - HTTP streaming
// - convert between time bases?
// - support embedded images

/*
#cgo pkg-config: libavformat libavcodec libavutil libswresample

#include <libavformat/avformat.h>
#include <libavutil/audio_fifo.h>
#include <libswresample/swresample.h>
#include <stdlib.h>

static int avErrorEOF() {
	return AVERROR_EOF;
}

static int avErrorEAGAIN() {
	return AVERROR(EAGAIN);
}

static char const* emptyString() {
	return "";
}
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

	var src *AudioSource
	var err error
	if src, err = newAudioFileSource(os.Args[1]); err != nil {
		panic(err)
	}
	defer src.Destroy()

	src.dumpFormat()

	var sink *AudioSink
	if sink, err = newAudioFileSink(os.Args[2],
		&AudioSinkOptions{
			Channels: 2, SampleRate: 44100,
		},
	); err != nil {
		panic(err)
	}
	defer sink.Destroy()

	var fifo *AudioFIFO
	if fifo, err = newAudioFIFO(sink); err != nil {
		panic(err)
	}
	defer fifo.Destroy()

	var resampler *AudioResampler
	if resampler, err = newAudioResampler(); err != nil {
		panic(err)
	}
	defer resampler.Destroy()

	if err = resampler.Setup(src, sink); err != nil {
		panic(err)
	}

	// TODO need to prevent over-buffering or sending too much data before
	//      it can be played
	// - for now: rely on client's buffer being small(ish)?
	//   - fine for some players (foobar), not for others (browsers)
	// - later: throttle encoding speed based on playback speed
	//          (available in packet after av_read_frame())

	done := false
	for !done {
		outFrameSize := int(sink.codecCtx.frame_size)

		for fifo.Size() < outFrameSize {
			if done, err = src.decodeFrames(fifo, resampler); err != nil {
				panic(err)
			}
			if done {
				break
			}
		}

		for fifo.Size() >= outFrameSize || (done && fifo.Size() > 0) {
			if err = sink.encodeFrames(fifo); err != nil {
				panic(err)
			}
		}
	}
	sink.flush()
	sink.writeTrailer()
}

func (packet *C.AVPacket) Init() {
	C.av_init_packet(packet)
	packet.data = nil
	packet.size = 0
}

func (src *AudioSource) decodeFrames(
	fifo *AudioFIFO,
	rs *AudioResampler,
) (bool /*finished*/, error) {
	// XXX do we really have to allocate a new frame every time? (no)
	var frame *C.AVFrame
	if frame = C.av_frame_alloc(); frame == nil {
		return false, fmt.Errorf("failed to allocate input frame")
	}
	defer C.av_frame_free(&frame)

	var packet C.AVPacket
	packet.Init()

	if err := C.av_read_frame(src.formatCtx, &packet); err == C.avErrorEOF() {
		// return?
	} else if err < 0 {
		return false, fmt.Errorf("failed to read frame: %v", avErr2Str(err))
	}
	defer C.av_packet_unref(&packet)

	if err := C.avcodec_send_packet(src.codecCtx, &packet); err < 0 {
		return false, fmt.Errorf("failed to send packet to decoder: %v", avErr2Str(err))
	}

	for {
		if err := C.avcodec_receive_frame(src.codecCtx, frame); err == C.avErrorEAGAIN() {
			return false, nil
		} else if err == C.avErrorEOF() {
			return true, nil
		} else if err < 0 {
			return false, fmt.Errorf("failed to receive frame from decoder: %v", avErr2Str(err))
		}

		if rs != nil {
			if _, err := rs.convert(frame.extended_data, frame.nb_samples, fifo); err != nil {
				return false, err
			}
		} else if C.av_audio_fifo_write(
			fifo.fifo, (*unsafe.Pointer)(unsafe.Pointer(frame.extended_data)), frame.nb_samples,
		) < frame.nb_samples {
			return false, fmt.Errorf("failed to write data to FIFO")
		}
	}
}

func (sink *AudioSink) encodeFrames(fifo *AudioFIFO) error {
	var frameSize C.int
	if C.int(fifo.Size()) < sink.codecCtx.frame_size {
		frameSize = C.int(fifo.Size())
	} else {
		frameSize = sink.codecCtx.frame_size
	}

	// XXX do we really have to allocate a new frame every time?
	var frame *C.AVFrame
	if frame = C.av_frame_alloc(); frame == nil {
		return fmt.Errorf("failed to allocate input frame")
	}
	defer C.av_frame_free(&frame)

	frame.nb_samples = frameSize
	frame.channel_layout = sink.codecCtx.channel_layout
	frame.format = C.int(sink.codecCtx.sample_fmt)
	frame.sample_rate = sink.codecCtx.sample_rate

	if err := C.av_frame_get_buffer(frame, 0); err < 0 {
		return fmt.Errorf("failed to allocate output frame buffer: %s", avErr2Str(err))
	}

	if C.av_audio_fifo_read(
		fifo.fifo, (*unsafe.Pointer)(unsafe.Pointer(&frame.data[0])), frameSize,
	) < frameSize {
		return fmt.Errorf("failed to read from FIFO")
	}

	frame.pts = sink.runningTime
	sink.runningTime += C.int64_t(frameSize)

	if err := C.avcodec_send_frame(sink.codecCtx, frame); err < 0 {
		return fmt.Errorf("failed to encode frame: %s", avErr2Str(err))
	}

	return sink.writeFrames()
}

func (sink *AudioSink) flush() error {
	if err := C.avcodec_send_frame(sink.codecCtx, nil); err < 0 {
		return fmt.Errorf("failed to encode NULL frame: %s", avErr2Str(err))
	}
	return sink.writeFrames()
}

func (sink *AudioSink) writeFrames() error {
	var packet C.AVPacket
	packet.Init()
	defer C.av_packet_unref(&packet)

	for {
		// calls av_packet_unref()
		if err := C.avcodec_receive_packet(sink.codecCtx, &packet); err == C.avErrorEAGAIN() {
			break
		} else if err == C.avErrorEOF() {
			return fmt.Errorf("unexpected EOF from encoder (might be normal)")
		} else if err < 0 {
			return fmt.Errorf("failed to receive packet from encoder: %v", avErr2Str(err))
		}

		if err := C.av_write_frame(sink.formatCtx, &packet); err < 0 {
			return fmt.Errorf("failed to write frame: %s", avErr2Str(err))
		}
	}
	return nil
}

func (sink *AudioSink) writeTrailer() error {
	if err := C.av_write_trailer(sink.formatCtx); err < 0 {
		return fmt.Errorf("failed to write trailer: %s", avErr2Str(err))
	}
	return nil
}

func avErr2Str(code C.int) string {
	var buffer [C.AV_ERROR_MAX_STRING_SIZE]C.char
	if C.av_strerror(code, &buffer[0], C.AV_ERROR_MAX_STRING_SIZE) < 0 {
		return "Unknown error"
	}
	return C.GoString(&buffer[0])
}

type AudioSource struct {
	Path string
	Tags map[string]string

	formatCtx *C.struct_AVFormatContext
	codecCtx  *C.struct_AVCodecContext
}

func (src *AudioSource) Destroy() {
	if src.codecCtx != nil {
		C.avcodec_free_context(&src.codecCtx)
	}
	if src.formatCtx != nil {
		C.avformat_close_input(&src.formatCtx)
	}
}

func (src *AudioSource) dumpFormat() {
	cPath := C.CString(src.Path)
	defer C.free(unsafe.Pointer(cPath))
	C.av_dump_format(src.formatCtx, 0, cPath, 0)
}

func newAudioFileSource(path string) (*AudioSource, error) {
	success := false
	src := AudioSource{Path: path}
	defer func() {
		if !success {
			src.Destroy()
		}
	}()

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	// open file
	if err := C.avformat_open_input(&src.formatCtx, cPath, nil, nil); err < 0 {
		return nil, fmt.Errorf("failed to open file: %v", avErr2Str(err))
	}

	// gather streams
	if err := C.avformat_find_stream_info(src.formatCtx, nil); err < 0 {
		return nil, fmt.Errorf("failed to find stream info: %v", avErr2Str(err))
	}

	// find first audio stream
	var audioStream *C.struct_AVStream
	{
		streamCount := src.formatCtx.nb_streams
		streams := (*[1 << 30]*C.struct_AVStream)(unsafe.Pointer(src.formatCtx.streams))[:streamCount:streamCount]
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

	if src.codecCtx = C.avcodec_alloc_context3(codec); src.codecCtx == nil {
		return nil, errors.New("failed to allocate decoding context")
	}

	if err := C.avcodec_parameters_to_context(src.codecCtx, audioStream.codecpar); err < 0 {
		return nil, fmt.Errorf("failed to copy codec parameters: %v", avErr2Str(err))
	}

	if err := C.avcodec_open2(src.codecCtx, codec, nil); err < 0 {
		return nil, fmt.Errorf("failed to open decoder: %v", avErr2Str(err))
	}

	src.Tags = make(map[string]string)
	{
		var entry *C.struct_AVDictionaryEntry
		for {
			if entry = C.av_dict_get(
				src.formatCtx.metadata, C.emptyString(), entry, C.AV_DICT_IGNORE_SUFFIX,
			); entry != nil {
				src.Tags[C.GoString(entry.key)] = C.GoString(entry.value)
			} else {
				break
			}
		}
	}

	success = true
	return &src, nil
}

type AudioSink struct {
	ioCtx     *C.struct_AVIOContext
	formatCtx *C.struct_AVFormatContext
	codecCtx  *C.struct_AVCodecContext

	runningTime C.int64_t
}

func (sink *AudioSink) Destroy() {
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

type AudioSinkOptions struct {
	Channels   uint
	SampleRate uint

	// TODO:
	// sample format
	// encoding
	// dictionary(?) of encoding-specific options (bit rate, etc.)
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
	if err := C.avio_open(&sink.ioCtx, cPath, C.AVIO_FLAG_WRITE); err < 0 {
		return nil, fmt.Errorf("failed to open file: %v", avErr2Str(err))
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

	codec := C.avcodec_find_encoder(C.AV_CODEC_ID_FLAC)
	if codec == nil {
		return nil, fmt.Errorf("failed to find output encoder")
	}

	if sink.codecCtx = C.avcodec_alloc_context3(codec); sink.codecCtx == nil {
		return nil, fmt.Errorf("failed to allocate encoding context")
	}

	sink.codecCtx.channels = C.int(options.Channels)
	sink.codecCtx.channel_layout = C.uint64_t(C.av_get_default_channel_layout(sink.codecCtx.channels))
	sink.codecCtx.sample_rate = C.int(options.SampleRate)
	sink.codecCtx.sample_fmt = *codec.sample_fmts // arbitrarily choose first supported format
	sink.codecCtx.time_base = stream.time_base

	// some container formats (like MP4) require global headers to be present.
	// mark the encoder so that it behaves accordingly
	if (sink.formatCtx.oformat.flags & C.AVFMT_GLOBALHEADER) != 0 {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_GLOBAL_HEADER
	}

	// open the encoder for the audio stream to use it later
	if err := C.avcodec_open2(sink.codecCtx, codec, nil); err < 0 {
		return nil, fmt.Errorf("failed to open output codec: %v", avErr2Str(err))
	}

	if err := C.avcodec_parameters_from_context(stream.codecpar, sink.codecCtx); err < 0 {
		return nil, fmt.Errorf("failed to initialize stream parameters")
	}

	if err := C.avformat_write_header(sink.formatCtx, nil); err < 0 {
		return nil, fmt.Errorf("failed to write header: %v", avErr2Str(err))
	}

	success = true
	return &sink, nil
}

type AudioFIFO struct {
	fifo *C.AVAudioFifo
}

func (fifo *AudioFIFO) Destroy() {
	if fifo.fifo != nil {
		C.av_audio_fifo_free(fifo.fifo)
		fifo.fifo = nil
	}
}

func newAudioFIFO(sink *AudioSink) (*AudioFIFO, error) {
	fifo := AudioFIFO{}

	fifo.fifo = C.av_audio_fifo_alloc(sink.codecCtx.sample_fmt, sink.codecCtx.channels, 1)
	if fifo.fifo == nil {
		return nil, fmt.Errorf("failed to allocate FIFO")
	}
	return &fifo, nil
}

func (fifo *AudioFIFO) Size() int {
	return int(C.av_audio_fifo_size(fifo.fifo))
}

type AudioResampler struct {
	swr            *C.struct_SwrContext
	buffer         **C.uint8_t
	bufferSamples  C.int
	bufferChannels C.int
	bufferFormat   int32
}

func (rs *AudioResampler) Destroy() {
	if rs.swr != nil {
		C.swr_free(&rs.swr)
	}
	rs.destroyBuffer()
}

func (rs *AudioResampler) destroyBuffer() {
	if rs.buffer != nil {
		C.av_freep(unsafe.Pointer(rs.buffer))  // free planes
		C.av_freep(unsafe.Pointer(&rs.buffer)) // free array of plane pointers
	}
}

func newAudioResampler() (*AudioResampler, error) {
	rs := AudioResampler{}

	if rs.swr = C.swr_alloc(); rs.swr == nil {
		return nil, fmt.Errorf("failed to allocate resampler")
	}
	return &rs, nil
}

func (rs *AudioResampler) Setup(
	src *AudioSource,
	sink *AudioSink,
) error {
	const defaultBufferSamples = 4096

	rs.swr = C.swr_alloc_set_opts(
		rs.swr,
		C.int64_t(sink.codecCtx.channel_layout),
		sink.codecCtx.sample_fmt,
		sink.codecCtx.sample_rate,
		C.int64_t(src.codecCtx.channel_layout),
		src.codecCtx.sample_fmt,
		src.codecCtx.sample_rate,
		0, nil, // logging offset and context
	)

	if err := C.swr_init(rs.swr); err < 0 {
		return fmt.Errorf("failed to initialize resampler: %v", avErr2Str(err))
	}

	rs.destroyBuffer()
	rs.bufferSamples = C.int(defaultBufferSamples)
	rs.bufferChannels = sink.codecCtx.channels
	rs.bufferFormat = sink.codecCtx.sample_fmt

	var lineSize C.int
	if err := C.av_samples_alloc_array_and_samples(
		&rs.buffer, &lineSize, rs.bufferChannels, rs.bufferSamples, rs.bufferFormat, 0,
	); err < 0 {
		return fmt.Errorf("failed to allocate sample buffer: %v", avErr2Str(err))
	}
	return nil
}

func (rs *AudioResampler) growBuffer(sampleCount C.int) error {
	if rs.bufferSamples <= sampleCount {
		return nil
	}

	C.av_freep(unsafe.Pointer(rs.buffer))

	var lineSize C.int
	if err := C.av_samples_alloc(
		rs.buffer, &lineSize, rs.bufferChannels, sampleCount, rs.bufferFormat, 0,
	); err < 0 {
		return fmt.Errorf("failed to allocate sample buffer: %v", avErr2Str(err))
	}

	rs.bufferSamples = sampleCount
	return nil
}

func (rs *AudioResampler) convert(
	in **C.uint8_t,
	inSamples C.int,
	out *AudioFIFO,
) (C.int /* samples written */, error) {
	if rs.buffer == nil {
		return 0, fmt.Errorf("convert() called without Setup()")
	}

	if maxOutSamples := C.swr_get_out_samples(rs.swr, inSamples); maxOutSamples >= 0 {
		if err := rs.growBuffer(maxOutSamples); err != nil {
			return 0, err
		}
	} else if maxOutSamples < 0 {
		return 0, fmt.Errorf(
			"failed to calculate output buffer size: %v", avErr2Str(maxOutSamples))
	}

	outSamples := C.swr_convert(rs.swr, rs.buffer, rs.bufferSamples, in, inSamples)
	if outSamples < 0 {
		return 0, fmt.Errorf("failed to convert samples: %v", avErr2Str(outSamples))
	}

	writtenSamples := C.av_audio_fifo_write(
		out.fifo, (*unsafe.Pointer)(unsafe.Pointer(rs.buffer)), outSamples)
	if writtenSamples < outSamples {
		return writtenSamples, fmt.Errorf("failed to write data to FIFO")
	}
	return writtenSamples, nil
}
