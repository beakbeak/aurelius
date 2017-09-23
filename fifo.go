package main

/*
#cgo pkg-config: libavutil

#include <libavutil/audio_fifo.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

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

func (fifo *AudioFIFO) read(
	data *unsafe.Pointer,
	sampleCount C.int,
) C.int {
	return C.av_audio_fifo_read(fifo.fifo, data, sampleCount)
}

func (fifo *AudioFIFO) write(
	data *unsafe.Pointer,
	sampleCount C.int,
) C.int {
	return C.av_audio_fifo_write(fifo.fifo, data, sampleCount)
}
