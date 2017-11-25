package main

import (
	"log"
	"net/http"
	"sb/aurelius/aurelib"
)

func stream(
	w http.ResponseWriter,
	req *http.Request,
) {
	reject := func(format string, args ...interface{}) {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf(format, args...)
	}

	options := aurelib.NewSinkOptions()
	options.Codec = "pcm_s16le"

	var sink *aurelib.BufferSink
	var err error
	if sink, err = aurelib.NewBufferSink("wav", options); err != nil {
		reject("failed to create sink: %v\n", err)
		return
	}
	defer sink.Destroy()

	var frames chan aurelib.Frame
	if frames, err = player.AddOutput(sink.StreamInfo(), sink.FrameSize()); err != nil {
		reject("failed to add output: %v\n", err)
		return
	}
	defer player.RemoveOutput(frames)

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Connection", "close")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	writeBuffer := func() error {
		buffer := sink.Buffer()
		if len(buffer) == 0 {
			return nil
		}

		count, err := w.Write(buffer)
		w.(http.Flusher).Flush()

		if count > 0 {
			sink.Drain(uint(count))
		}
		noise.Printf("wrote %v bytes\n", count)
		return err
	}

	for frame := range frames {
		if _, err = sink.Encode(frame); err != nil {
			log.Printf("failed to encode frame: %v\n", err)
			break
		}
		if err = writeBuffer(); err != nil {
			log.Printf("failed to write buffer: %v\n", err)
			break
		}
	}

	if err = aurelib.FlushSink(sink); err != nil {
		log.Printf("failed to flush sink: %v\n", err)
	} else if err = writeBuffer(); err != nil {
		log.Printf("failed to write buffer: %v\n", err)
	}
}
