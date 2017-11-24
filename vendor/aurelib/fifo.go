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
	info StreamInfo
}

func (fifo *Fifo) Destroy() {
	if fifo.fifo != nil {
		C.av_audio_fifo_free(fifo.fifo)
		fifo.fifo = nil
	}
}

func NewFifo(info StreamInfo) (*Fifo, error) {
	fifo := Fifo{info: info}

	fifo.fifo = C.av_audio_fifo_alloc(
		info.sampleFormat, info.channelCount(), 1)
	if fifo.fifo == nil {
		return nil, fmt.Errorf("failed to allocate FIFO")
	}
	return &fifo, nil
}

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

// returned Frame must be Frame.Destroy()ed or Sink.Encode()ed
func (fifo *Fifo) ReadFrame(frameSize uint) (Frame, error) {
	if fifo.Size() <= 0 {
		return Frame{}, nil
	}

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
	frame.frame.channel_layout = C.uint64_t(fifo.info.channelLayout)
	frame.frame.format = C.int(fifo.info.sampleFormat)
	frame.frame.sample_rate = C.int(fifo.info.SampleRate)

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
