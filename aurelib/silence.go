package aurelib

/*
#cgo pkg-config: libavutil

#include <libavutil/samplefmt.h>
#include <libavutil/channel_layout.h>

static uint64_t avChLayoutStereo() {
	return AV_CH_LAYOUT_STEREO;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type SilenceSource struct {
	decodeCalled bool
	tags         map[string]string
	streamInfo   StreamInfo
	buffer       **C.uint8_t
}

const silenceBufferSize = 4096

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

func (s *SilenceSource) Destroy() {
	if s.buffer != nil {
		C.av_freep(unsafe.Pointer(s.buffer))  // free planes
		C.av_freep(unsafe.Pointer(&s.buffer)) // free array of plane pointers
	}
}

func (*SilenceSource) ReplayGain(
	mode ReplayGainMode,
	preventClipping bool,
) float64 {
	return 1
}

func (s *SilenceSource) Tags() map[string]string {
	return s.tags
}

func (s *SilenceSource) StreamInfo() StreamInfo {
	return s.streamInfo
}

func (s *SilenceSource) Decode() (error, bool) {
	s.decodeCalled = true
	return nil, true
}

func (s *SilenceSource) ReceiveFrame() (ReceiveFrameStatus, error) {
	if s.decodeCalled {
		s.decodeCalled = false
		return ReceiveFrameCopyAndCallAgain, nil
	}
	return ReceiveFrameEmpty, nil
}

func (s *SilenceSource) FrameSize() uint {
	return silenceBufferSize
}

func (s *SilenceSource) CopyFrame(
	fifo *Fifo,
	rs *Resampler,
) error {
	if rs != nil {
		if _, err := rs.convert(s.buffer, silenceBufferSize, fifo); err != nil {
			return err
		}
	} else if fifo.write(
		(*unsafe.Pointer)(unsafe.Pointer(s.buffer)), silenceBufferSize,
	) < silenceBufferSize {
		return fmt.Errorf("failed to write data to FIFO")
	}
	return nil
}
