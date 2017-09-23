package aurelib

/*
#cgo pkg-config: libavformat libavcodec libavutil libswresample

#include <libavutil/audio_fifo.h>
#include <libavutil/opt.h>
#include <libswresample/swresample.h>

static char const* strRematrixVolume() {
	return "rematrix_volume";
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type Resampler struct {
	swr            *C.struct_SwrContext
	buffer         **C.uint8_t
	bufferSamples  C.int
	bufferChannels C.int
	bufferFormat   int32
}

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

func NewResampler() (*Resampler, error) {
	rs := Resampler{}

	if rs.swr = C.swr_alloc(); rs.swr == nil {
		return nil, fmt.Errorf("failed to allocate resampler")
	}
	return &rs, nil
}

func (rs *Resampler) Setup(
	src *Source,
	sink *Sink,
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

func (rs *Resampler) convert(
	in **C.uint8_t,
	inSamples C.int,
	out *Fifo,
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

	writtenSamples := out.write((*unsafe.Pointer)(unsafe.Pointer(rs.buffer)), outSamples)
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
