package aurelib

/*
#cgo pkg-config: libavutil

#include <libavutil/audio_fifo.h>
#include <libavformat/avformat.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// A Fifo is a first-in-first-out audio buffer. It is used to store audio data
// generated by a Source, which is then broken into Frames and consumed by a
// Sink.
type Fifo struct {
	fifo       *C.AVAudioFifo
	streamInfo StreamInfo
}

// Destroy frees C heap memory used by the Fifo.
func (fifo *Fifo) Destroy() {
	if fifo.fifo != nil {
		C.av_audio_fifo_free(fifo.fifo)
		fifo.fifo = nil
	}
}

// NewFifo creates a new Fifo object that accepts audio data in the format
// described by the supplied StreamInfo.
//
// The Fifo is backed by a heap-allocated C data structure, so it must be
// destroyed with Destroy before it is discarded.
func NewFifo(info StreamInfo) (*Fifo, error) {
	fifo := Fifo{streamInfo: info}

	fifo.fifo = C.av_audio_fifo_alloc(
		info.sampleFormat, info.channelCount(), 1)
	if fifo.fifo == nil {
		return nil, fmt.Errorf("failed to allocate FIFO")
	}
	return &fifo, nil
}

// Size returns the number of samples available for reading from the Fifo.
func (fifo *Fifo) Size() uint {
	return uint(C.av_audio_fifo_size(fifo.fifo))
}

func (fifo *Fifo) read(
	data *unsafe.Pointer,
	sampleCount C.int,
) C.int {
	return C.av_audio_fifo_read(fifo.fifo, data, sampleCount)
}

func (fifo *Fifo) write(
	data *unsafe.Pointer,
	sampleCount C.int,
) C.int {
	return C.av_audio_fifo_write(fifo.fifo, data, sampleCount)
}

// ReadFrame removes at most maxFrameSize samples from the Fifo's audio buffer
// and returns them in a Frame object.
//
// The returned Frame must be destroyed with Frame.Destroy or passed to a
// function that takes ownership, such as Sink.Encode.
func (fifo *Fifo) ReadFrame(maxFrameSize uint) (Frame, error) {
	if fifo.Size() == 0 {
		return Frame{}, nil
	}

	frameSize := maxFrameSize
	if fifo.Size() < frameSize {
		frameSize = fifo.Size()
	}

	success := false
	frame := Frame{}
	if frame.frame = C.av_frame_alloc(); frame.frame == nil {
		return Frame{}, fmt.Errorf("failed to allocate input frame")
	}
	defer func() {
		if !success {
			frame.Destroy()
		}
	}()

	frame.frame.nb_samples = C.int(frameSize)
	frame.frame.format = C.int(fifo.streamInfo.sampleFormat)
	frame.frame.sample_rate = C.int(fifo.streamInfo.SampleRate)
	withChannelLayout(fifo.streamInfo.channelLayout, func(layout *C.AVChannelLayout) {
		C.av_channel_layout_copy(&frame.frame.ch_layout, layout)
	})

	if err := C.av_frame_get_buffer(frame.frame, 0); err < 0 {
		return Frame{}, fmt.Errorf("failed to allocate output frame buffer: %s", avErr2Str(err))
	}

	if fifo.read(
		(*unsafe.Pointer)(unsafe.Pointer(&frame.frame.data[0])), C.int(frameSize),
	) < C.int(frameSize) {
		return Frame{}, fmt.Errorf("failed to read from FIFO")
	}

	success = true
	frame.Size = frameSize
	return frame, nil
}
