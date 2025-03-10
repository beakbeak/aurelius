package aurelib

/*
#cgo pkg-config: libavutil

#include <libavutil/samplefmt.h>
#include <libavutil/channel_layout.h>
#include <libavutil/mem.h>

static uint64_t
avChLayoutStereo() {
	return AV_CH_LAYOUT_STEREO;
}
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

// A SilenceSource produces silent audio data.
type SilenceSource struct {
	decodeCalled bool
	tags         map[string]string
	streamInfo   StreamInfo
	buffer       **C.uint8_t
}

const silenceBufferSize = 4096

// NewSilenceSource creates a new SilenceSource.
//
// The Source is backed by a heap-allocated C data structure, so it must be
// destroyed with Destroy before it is discarded.
func NewSilenceSource() (*SilenceSource, error) {
	success := false
	streamInfo := StreamInfo{
		SampleRate:    44100,
		sampleFormat:  C.AV_SAMPLE_FMT_S16,
		channelLayout: int64(C.avChLayoutStereo()),
	}
	s := SilenceSource{
		tags:       make(map[string]string),
		streamInfo: streamInfo,
	}
	defer func() {
		if !success {
			s.Destroy()
		}
	}()

	channelCount := streamInfo.channelCount()

	var lineSize C.int
	if err := C.av_samples_alloc_array_and_samples(
		&s.buffer, &lineSize, channelCount, silenceBufferSize, streamInfo.sampleFormat, 0,
	); err < 0 {
		return nil, fmt.Errorf("failed to allocate sample buffer: %v", avErr2Str(err))
	}

	if err := C.av_samples_set_silence(
		s.buffer, 0, silenceBufferSize, channelCount, streamInfo.sampleFormat,
	); err < 0 {
		return nil, fmt.Errorf("failed to generate silence: %v", avErr2Str(err))
	}

	success = true
	return &s, nil
}

// Destroy frees any resources held by the Source so that it may be discarded.
func (s *SilenceSource) Destroy() {
	if s.buffer != nil {
		C.av_freep(unsafe.Pointer(s.buffer))  // free planes
		C.av_freep(unsafe.Pointer(&s.buffer)) // free array of plane pointers
	}
}

// ReplayGain returns 1.
func (*SilenceSource) ReplayGain(
	mode ReplayGainMode,
	preventClipping bool,
) float64 {
	return 1
}

// Tags returns an empty map.
func (s *SilenceSource) Tags() map[string]string {
	return s.tags
}

// StreamInfo returns an object describing the format of audio data produced by
// the Source.
func (s *SilenceSource) StreamInfo() StreamInfo {
	return s.streamInfo
}

// Decode transfers an encoded packet from the input to the decoder. It must be
// followed by one or more calls to ReceiveFrame.
//
// When a non-nil error is returned, the 'recoverable' return value will be true
// if the error is recoverable and Decode can be safely called again.
func (s *SilenceSource) Decode() (error, bool) {
	s.decodeCalled = true
	return nil, true
}

// ReceiveFrame receives a decoded frame from the decoder.
//
// The returned ReceiveFrameStatus indicates how the code should proceed after
// the call to ReceiveFrame.
func (s *SilenceSource) ReceiveFrame() (ReceiveFrameStatus, error) {
	if s.decodeCalled {
		s.decodeCalled = false
		return ReceiveFrameCopyAndCallAgain, nil
	}
	return ReceiveFrameEmpty, nil
}

// FrameSize returns the number of samples in the last frame received by a call
// to ReceiveFrame.
func (s *SilenceSource) FrameSize() uint {
	return silenceBufferSize
}

// FrameStartTime returns the stream time offset of the start of the last frame
// received by a call to ReceiveFrame.
func (s *SilenceSource) FrameStartTime() uint {
	return 0
}

// CopyFrame copies the data received by ReceiveFrame to the supplied Fifo.
//
// CopyFrame and ResampleFrame may be called multiple times to supply data to
// multiple Fifos.
func (s *SilenceSource) CopyFrame(fifo *Fifo) error {
	if fifo.write(
		(*unsafe.Pointer)(unsafe.Pointer(s.buffer)), silenceBufferSize,
	) < silenceBufferSize {
		return fmt.Errorf("failed to write data to FIFO")
	}
	return nil
}

// ResampleFrame resamples the data received by ReceiveFrame and passes it to
// the supplied Fifo.
//
// CopyFrame and ResampleFrame may be called multiple times to supply data to
// multiple Fifos.
func (s *SilenceSource) ResampleFrame(
	rs *Resampler,
	fifo *Fifo,
) error {
	if _, err := rs.resample(s.buffer, silenceBufferSize, fifo); err != nil {
		return err
	}
	return nil
}

// Seek causes streaming to continue from the given offset relative to the
// beginning of the audio stream.
func (s *SilenceSource) SeekTo(offset time.Duration) error {
	return nil
}
