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

type Fifo struct {
	fifo *C.AVAudioFifo
}

func (fifo *Fifo) Destroy() {
	if fifo.fifo != nil {
		C.av_audio_fifo_free(fifo.fifo)
		fifo.fifo = nil
	}
}

func NewFifo(sink Sink) (*Fifo, error) {
	fifo := Fifo{}

	fifo.fifo = C.av_audio_fifo_alloc(
		sink.codecContext().sample_fmt, sink.codecContext().channels, 1)
	if fifo.fifo == nil {
		return nil, fmt.Errorf("failed to allocate FIFO")
	}
	return &fifo, nil
}

func (fifo *Fifo) Size() int {
	return int(C.av_audio_fifo_size(fifo.fifo))
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

// returned Frame must be Frame.Destroy()ed or Sink.Encode()ed
func (fifo *Fifo) ReadFrame(sink Sink) (Frame, error) {
	if fifo.Size() <= 0 {
		return Frame{}, nil
	}

	frameSize := sink.FrameSize()
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
	frame.frame.channel_layout = sink.codecContext().channel_layout
	frame.frame.format = C.int(sink.codecContext().sample_fmt)
	frame.frame.sample_rate = sink.codecContext().sample_rate

	if err := C.av_frame_get_buffer(frame.frame, 0); err < 0 {
		return Frame{}, fmt.Errorf("failed to allocate output frame buffer: %s", avErr2Str(err))
	}

	if fifo.read(
		(*unsafe.Pointer)(unsafe.Pointer(&frame.frame.data[0])), C.int(frameSize),
	) < C.int(frameSize) {
		return Frame{}, fmt.Errorf("failed to read from FIFO")
	}

	success = true
	frame.Size = uint(frameSize)
	return frame, nil
}
