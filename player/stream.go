package player

import (
	"net/http"
	"sb/aurelius/aurelib"
	"sb/aurelius/util"
)

func (player *Player) Stream(
	w http.ResponseWriter,
	req *http.Request,
) {
	reject := func(format string, args ...interface{}) {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf(format, args...)
	}

	options := aurelib.NewSinkOptions()
	options.Codec = "pcm_s16le"

	sink, err := aurelib.NewBufferSink("wav", options)
	if err != nil {
		reject("failed to create sink: %v\n", err)
		return
	}
	defer sink.Destroy()

	frames, err := player.AddOutput(sink.StreamInfo(), sink.FrameSize())
	if err != nil {
		reject("failed to add output: %v\n", err)
		return
	}
	defer player.RemoveOutput(frames)

	w.Header().Set("Content-Type", "audio/wav")
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
		util.Noise.Printf("wrote %v bytes\n", count)
		return err
	}

	for frame := range frames {
		if _, err = sink.Encode(frame); err != nil {
			util.Debug.Printf("failed to encode frame: %v\n", err)
			break
		}
		if err = writeBuffer(); err != nil {
			util.Debug.Printf("failed to write buffer: %v\n", err)
			break
		}
	}

	if err = aurelib.FlushSink(sink); err != nil {
		util.Debug.Printf("failed to flush sink: %v\n", err)
	} else if err = writeBuffer(); err != nil {
		util.Debug.Printf("failed to write buffer: %v\n", err)
	}
}
