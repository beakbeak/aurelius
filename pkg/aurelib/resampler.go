package aurelib

/*
#cgo pkg-config: libavformat libavcodec libavutil libswresample

#include <libavutil/audio_fifo.h>
#include <libavutil/opt.h>
#include <libavutil/channel_layout.h>
#include <libswresample/swresample.h>

static char const*
strRematrixVolume() {
	return "rematrix_volume";
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// A Resampler converts unencoded audio data between different sample rates and
// formats. It is used by Source.ResampleFrame.
type Resampler struct {
	swr            *C.SwrContext
	buffer         **C.uint8_t
	bufferSamples  C.int
	bufferChannels C.int
	bufferFormat   int32
}

// Destroy frees C heap memory used by the Resampler.
func (rs *Resampler) Destroy() {
	if rs.swr != nil {
		C.swr_free(&rs.swr)
	}
	rs.destroyBuffer()
}

func (rs *Resampler) destroyBuffer() {
	if rs.buffer != nil {
		C.av_freep(unsafe.Pointer(rs.buffer))  // free planes
		C.av_freep(unsafe.Pointer(&rs.buffer)) // free array of plane pointers
	}
}

// NewResampler creates a new Resampler. Before first use, it must be configured
// with the Setup method.
//
// The Resampler is backed by a heap-allocated C data structure, so it must be
// destroyed with Destroy before it is discarded.
func NewResampler() (*Resampler, error) {
	rs := Resampler{}

	if rs.swr = C.swr_alloc(); rs.swr == nil {
		return nil, fmt.Errorf("failed to allocate resampler")
	}
	return &rs, nil
}

// Setup configures the Resampler to convert audio data from the format
// described by srcInfo to the format described by sinkInfo.
//
// The loudness of the converted data will be scaled by the 'volume' argument. A
// value of 1 will prevent the volume from being changed.
func (rs *Resampler) Setup(
	srcInfo StreamInfo,
	sinkInfo StreamInfo,
	volume float64,
) error {
	const defaultBufferSamples = 4096

	var err error
	withChannelLayout(srcInfo.channelLayout, func(srcLayout *C.AVChannelLayout) {
		withChannelLayout(sinkInfo.channelLayout, func(sinkLayout *C.AVChannelLayout) {
			if avErr := C.swr_alloc_set_opts2(
				&rs.swr,
				sinkLayout,
				sinkInfo.sampleFormat,
				C.int(sinkInfo.SampleRate),
				srcLayout,
				srcInfo.sampleFormat,
				C.int(srcInfo.SampleRate),
				0, nil, // logging offset and context
			); avErr < 0 {
				err = fmt.Errorf("failed to init resampler: %s", avErr2Str(avErr))
			}
		})
	})
	if err != nil {
		return err
	}

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
	rs.bufferChannels = sinkInfo.channelCount()
	rs.bufferFormat = sinkInfo.sampleFormat

	var lineSize C.int
	if err := C.av_samples_alloc_array_and_samples(
		&rs.buffer, &lineSize, rs.bufferChannels, rs.bufferSamples, rs.bufferFormat, 0,
	); err < 0 {
		return fmt.Errorf("failed to allocate sample buffer: %v", avErr2Str(err))
	}
	return nil
}

func (rs *Resampler) growBuffer(sampleCount C.int) error {
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

func (rs *Resampler) resample(
	inBuffer **C.uint8_t,
	inSamples C.int,
	out *Fifo,
) (writtenSamples C.int, _ error) {
	if rs.buffer == nil {
		return 0, fmt.Errorf("resample() called without Setup()")
	}

	if maxOutSamples := C.swr_get_out_samples(rs.swr, inSamples); maxOutSamples >= 0 {
		if err := rs.growBuffer(maxOutSamples); err != nil {
			return 0, err
		}
	} else if maxOutSamples < 0 {
		return 0, fmt.Errorf(
			"failed to calculate output buffer size: %v", avErr2Str(maxOutSamples))
	}

	outSamples := C.swr_convert(rs.swr, rs.buffer, rs.bufferSamples, inBuffer, inSamples)
	if outSamples < 0 {
		return 0, fmt.Errorf("failed to convert samples: %v", avErr2Str(outSamples))
	}

	writtenSamples = out.write((*unsafe.Pointer)(unsafe.Pointer(rs.buffer)), outSamples)
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
			writtenSamples = out.write((*unsafe.Pointer)(unsafe.Pointer(rs.buffer)), outSamples)
			totalWrittenSamples += writtenSamples

			if writtenSamples < outSamples {
				return totalWrittenSamples, fmt.Errorf("failed to write data to FIFO")
			}
		}
	}
	return totalWrittenSamples, nil
}
