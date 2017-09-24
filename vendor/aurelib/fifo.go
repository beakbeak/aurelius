package aurelib

/*
#cgo pkg-config: libavutil

#include <libavutil/audio_fifo.h>
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
