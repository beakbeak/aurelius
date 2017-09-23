package main

// TODO:
// - figure out why timing is wrong when using MKV container
//   (maybe something to do with time base settings/conversion)
// - flesh out encoding & resampling options
//   - dictionary(?) of encoding-specific options (bit rate, etc.)
// - HTTP streaming
//   - need to prevent over-buffering or sending too much data before
//     it can be played
//     - for now: fixed-size buffer; rely on client to drain at appropriate speed
//     - later: throttle encoding speed based on playback speed
//              (available in packet after av_read_frame())
//       - *** this will be necessary for silence when paused ***
// - support embedded images

// WISHLIST:
// - replaygain preamp?
// - treat sections of a file as playlist entries (e.g. pieces of a long live set, a hidden track)
//   - can't use m3u anymore
// - tag editing

/*
#cgo pkg-config: libavformat libavcodec libavutil libswresample

#include <libavformat/avformat.h>
#include <libavutil/audio_fifo.h>
#include <libavutil/opt.h>
#include <libavutil/replaygain.h>
#include <libswresample/swresample.h>
#include <stdlib.h>

static int avErrorEOF() {
	return AVERROR_EOF;
}

static int avErrorEAGAIN() {
	return AVERROR(EAGAIN);
}

static char const* strEmpty() {
	return "";
}

static char const* strRematrixVolume() {
	return "rematrix_volume";
}
*/
import "C"
import (
	"errors"
	"fmt"
	"math"
	"os"
	"unsafe"
)

func main() {
	if len(os.Args) < 3 {
		panic("not enough arguments")
	}

	C.av_register_all()
	C.avformat_network_init()
	defer C.avformat_network_deinit()

	var resampler *AudioResampler
	var err error
	if resampler, err = newAudioResampler(); err != nil {
		panic(err)
	}
	defer resampler.Destroy()

	var sink *AudioSink
	if sink, err = newAudioFileSink(os.Args[len(os.Args)-1], nil); err != nil {
		panic(err)
	}
	defer sink.Destroy()

	var fifo *AudioFIFO
	if fifo, err = newAudioFIFO(sink); err != nil {
		panic(err)
	}
	defer fifo.Destroy()

	playFile := func(path string) error {
		var src *AudioSource
		var err error
		if src, err = newAudioFileSource(path); err != nil {
			return err
		}
		defer src.Destroy()

		src.dumpFormat()
		fmt.Println(src.Tags)

		if err := resampler.Setup(src, sink, src.ReplayGain(ReplayGainTrack, true)); err != nil {
			return err
		}

		done := false
		for !done {
			outFrameSize := sink.FrameSize()

			for fifo.Size() < outFrameSize {
				if done, err = src.decodeFrames(fifo, resampler); err != nil {
					return err
				}
				if done {
					break
				}
			}

			for fifo.Size() >= outFrameSize {
				if err = sink.encodeFrames(fifo); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, path := range os.Args[1 : len(os.Args)-1] {
		if err := playFile(path); err != nil {
			fmt.Printf("failed to play '%v': %v\n", path, err)
		}
	}

	if err = sink.flush(fifo); err != nil {
		fmt.Printf("failed to flush sink: %v\n", err)
	}
	if err = sink.writeTrailer(); err != nil {
		fmt.Printf("failed to write trailer: %v\n", err)
	}
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
	if src.frame == nil {
		if src.frame = C.av_frame_alloc(); src.frame == nil {
			return false, fmt.Errorf("failed to allocate input frame")
		}
	}
	defer C.av_frame_unref(src.frame)

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
		// calls av_frame_unref()
		if err := C.avcodec_receive_frame(src.codecCtx, src.frame); err == C.avErrorEAGAIN() {
			return false, nil
		} else if err == C.avErrorEOF() {
			return true, nil
		} else if err < 0 {
			return false, fmt.Errorf("failed to receive frame from decoder: %v", avErr2Str(err))
		}

		if rs != nil {
			if _, err := rs.convert(src.frame.extended_data, src.frame.nb_samples, fifo); err != nil {
				return false, err
			}
		} else if C.av_audio_fifo_write(
			fifo.fifo, (*unsafe.Pointer)(unsafe.Pointer(src.frame.extended_data)),
			src.frame.nb_samples,
		) < src.frame.nb_samples {
			return false, fmt.Errorf("failed to write data to FIFO")
		}
	}
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

	if C.av_audio_fifo_read(
		fifo.fifo, (*unsafe.Pointer)(unsafe.Pointer(&sink.frame.data[0])), C.int(frameSize),
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
	packet.Init()
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
	stream    *C.struct_AVStream

	frame *C.struct_AVFrame
}

func (src *AudioSource) Destroy() {
	if src.frame != nil {
		C.av_frame_free(&src.frame)
	}
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
	{
		streamCount := src.formatCtx.nb_streams
		streams := (*[1 << 30]*C.struct_AVStream)(unsafe.Pointer(src.formatCtx.streams))[:streamCount:streamCount]
		for _, stream := range streams {
			if stream.codecpar.codec_type == C.AVMEDIA_TYPE_AUDIO {
				src.stream = stream
				break
			}
		}
	}
	if src.stream == nil {
		return nil, errors.New("no audio streams")
	}

	codec := C.avcodec_find_decoder(src.stream.codecpar.codec_id)
	if codec == nil {
		return nil, errors.New("failed to find decoder")
	}

	if src.codecCtx = C.avcodec_alloc_context3(codec); src.codecCtx == nil {
		return nil, errors.New("failed to allocate decoding context")
	}

	if err := C.avcodec_parameters_to_context(src.codecCtx, src.stream.codecpar); err < 0 {
		return nil, fmt.Errorf("failed to copy codec parameters: %v", avErr2Str(err))
	}

	if err := C.avcodec_open2(src.codecCtx, codec, nil); err < 0 {
		return nil, fmt.Errorf("failed to open decoder: %v", avErr2Str(err))
	}

	src.Tags = make(map[string]string)

	gatherTagsFromDict := func(dict *C.struct_AVDictionary) {
		var entry *C.struct_AVDictionaryEntry
		for {
			if entry = C.av_dict_get(
				dict, C.strEmpty(), entry, C.AV_DICT_IGNORE_SUFFIX,
			); entry != nil {
				src.Tags[C.GoString(entry.key)] = C.GoString(entry.value)
			} else {
				break
			}
		}
	}
	gatherTagsFromDict(src.formatCtx.metadata)
	gatherTagsFromDict(src.stream.metadata)

	success = true
	return &src, nil
}

type ReplayGainMode int

const (
	ReplayGainTrack ReplayGainMode = iota
	ReplayGainAlbum
)

// ReplayGain returns the [0,1] volume scale factor that should be applied
// based on the audio stream's ReplayGain metadata and the supplied arguments.
func (src *AudioSource) ReplayGain(
	mode ReplayGainMode,
	preventClipping bool,
) float64 {
	var data *C.struct_AVPacketSideData
	for i := C.int(0); i < src.stream.nb_side_data; i++ {
		address := uintptr(unsafe.Pointer(src.stream.side_data))
		address += uintptr(i) * unsafe.Sizeof(*src.stream.side_data)
		localData := (*C.struct_AVPacketSideData)(unsafe.Pointer(address))

		if localData._type == C.AV_PKT_DATA_REPLAYGAIN {
			data = localData
			break
		}
	}
	if data == nil {
		return 1.
	}

	gainData := (*C.struct_AVReplayGain)(unsafe.Pointer(data.data))

	var gain, peak float64
	var gainValid, peakValid bool

	if gainData.track_gain != C.INT32_MIN {
		gain = float64(gainData.track_gain)
		gainValid = true
	}
	if gainData.track_peak != 0 {
		peak = float64(gainData.track_peak)
		peakValid = true
	}

	if (mode == ReplayGainAlbum || !gainValid) && gainData.album_gain != C.INT32_MIN {
		gain = float64(gainData.album_gain)
		gainValid = true
	}
	if (mode == ReplayGainAlbum || !peakValid) && gainData.album_peak != 0 {
		peak = float64(gainData.album_peak)
		peakValid = true
	}

	if !gainValid {
		return 1.
	}

	gain /= 100000.
	peak /= 100000.

	volume := math.Pow(10, gain/20.)
	if preventClipping && peakValid {
		invPeak := 1. / peak
		if volume > invPeak {
			volume = invPeak
		}
	}
	return volume
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

func (sink *AudioSink) FrameSize() int {
	value := sink.codecCtx.frame_size
	if value <= 0 {
		return 4096
	}
	return int(value)
}

type AudioSinkOptions struct {
	Channels      uint   // default 2
	ChannelLayout string // optional; overrides Channels
	SampleRate    uint   // default 44100
	SampleFormat  string // default is codec-dependent
	Codec         string // default "flac"
}

func (options *AudioSinkOptions) getChannels() (C.int, C.uint64_t) {
	var channels C.int
	var channelLayout C.uint64_t

	if options != nil && options.ChannelLayout != "" {
		channelLayoutName := C.CString(options.ChannelLayout)
		defer C.free(unsafe.Pointer(channelLayoutName))

		if channelLayout = C.av_get_channel_layout(channelLayoutName); channelLayout != 0 {
			channels = C.av_get_channel_layout_nb_channels(channelLayout)
		}
	}
	if channels == 0 {
		if options != nil && options.Channels != 0 {
			channels = C.int(options.Channels)
		} else {
			channels = 2
		}
		channelLayout = C.uint64_t(C.av_get_default_channel_layout(channels))
	}
	return channels, channelLayout
}

func (options *AudioSinkOptions) getSampleRate() C.int {
	if options != nil && options.SampleRate != 0 {
		return C.int(options.SampleRate)
	}
	return 44100
}

func (options *AudioSinkOptions) getSampleFormat(defaultFormat int32) int32 {
	if options != nil && options.SampleFormat != "" {
		formatName := C.CString(options.SampleFormat)
		defer C.free(unsafe.Pointer(formatName))

		return C.av_get_sample_fmt(formatName)
	}
	return defaultFormat
}

func (options *AudioSinkOptions) getCodec() (*C.struct_AVCodec, error) {
	var codecName string
	if options != nil && options.Codec != "" {
		codecName = options.Codec
	} else {
		codecName = "flac"
	}

	cCodecName := C.CString(codecName)
	defer C.free(unsafe.Pointer(cCodecName))

	codec := C.avcodec_find_encoder_by_name(cCodecName)
	if codec == nil {
		return nil, fmt.Errorf("failed to find output encoder '%v'", codecName)
	}
	return codec, nil
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
	stream.time_base.den = options.getSampleRate()

	var codec *C.struct_AVCodec
	var err error
	if codec, err = options.getCodec(); err != nil {
		return nil, err
	}

	if sink.codecCtx = C.avcodec_alloc_context3(codec); sink.codecCtx == nil {
		return nil, fmt.Errorf("failed to allocate encoding context")
	}

	sink.codecCtx.channels, sink.codecCtx.channel_layout = options.getChannels()

	sink.codecCtx.sample_rate = options.getSampleRate()
	sink.codecCtx.sample_fmt = options.getSampleFormat(*codec.sample_fmts)
	sink.codecCtx.time_base = stream.time_base

	// some container formats (like MP4) require global headers to be present.
	// mark the encoder so that it behaves accordingly
	if (sink.formatCtx.oformat.flags & C.AVFMT_GLOBALHEADER) != 0 {
		sink.codecCtx.flags |= C.AV_CODEC_FLAG_GLOBAL_HEADER
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
	volume float64,
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

	if err := C.av_opt_set_double(
		unsafe.Pointer(rs.swr), C.strRematrixVolume(), C.double(volume), 0,
	); err < 0 {
		return fmt.Errorf("failed to set resampler volume: %v", avErr2Str(err))
	}

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

	// flush resampler (probably unnecessary)
	totalWrittenSamples := writtenSamples
	for outSamples != 0 {
		outSamples = C.swr_convert(rs.swr, rs.buffer, rs.bufferSamples, nil, 0)
		if outSamples < 0 {
			return totalWrittenSamples,
				fmt.Errorf("failed to convert samples: %v", avErr2Str(outSamples))
		} else if outSamples > 0 {
			writtenSamples = C.av_audio_fifo_write(
				out.fifo, (*unsafe.Pointer)(unsafe.Pointer(rs.buffer)), outSamples)
			totalWrittenSamples += writtenSamples

			if writtenSamples < outSamples {
				return totalWrittenSamples, fmt.Errorf("failed to write data to FIFO")
			}
		}
	}
	return totalWrittenSamples, nil
}
