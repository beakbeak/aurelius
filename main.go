package main

// TODO:
// - HTTP streaming
//   - need to prevent over-buffering or sending too much data before
//     it can be played
//     - throttle encoding speed based on playback speed
//       (available in packet after av_read_frame())
// - play silence when nothing is left to play
// - figure out why timing is wrong when using MKV container
//   (maybe something to do with time base settings/conversion)
// - support embedded images

// WISHLIST:
// - replaygain preamp?
// - treat sections of a file as playlist entries (e.g. pieces of a long live set, a hidden track)
//   - can't use m3u anymore
// - tag editing
// - get replaygain info from RVA2 mp3 tag (requires another library dependency)

import (
	"aurelib"
	"log"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		panic("not enough arguments")
	}

	aurelib.Init()

	http.HandleFunc("/", stream)
	log.Fatal(http.ListenAndServe(":9090", nil))
}

func stream(
	w http.ResponseWriter,
	req *http.Request,
) {
	reject := func(format string, args ...interface{}) {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf(format, args...)
	}

	var resampler *aurelib.Resampler
	var err error
	if resampler, err = aurelib.NewResampler(); err != nil {
		reject("failed to create resampler: %v\n", err)
		return
	}
	defer resampler.Destroy()

	options := aurelib.NewSinkOptions()
	options.Codec = "pcm_s16le"

	var sink *aurelib.BufferSink
	if sink, err = aurelib.NewBufferSink("wav", options); err != nil {
		reject("failed to create sink: %v\n", err)
		return
	}
	defer sink.Destroy()

	var fifo *aurelib.Fifo
	if fifo, err = aurelib.NewFifo(sink); err != nil {
		reject("failed to create FIFO: %v\n", err)
		return
	}
	defer fifo.Destroy()

	writeBuffer := func() error {
		buffer := sink.Buffer()
		if len(buffer) == 0 {
			return nil
		}

		count, err := w.Write(sink.Buffer())
		if count > 0 {
			sink.Drain(uint(count))
		}
		log.Printf("wrote %v bytes\n", count)
		return err
	}

	playFile := func(path string) error {
		var src *aurelib.FileSource
		var err error
		if src, err = aurelib.NewFileSource(path); err != nil {
			return err
		}
		defer src.Destroy()

		src.DumpFormat()
		log.Println(src.Tags)

		if err := resampler.Setup(
			src, sink, src.ReplayGain(aurelib.ReplayGainTrack, true),
		); err != nil {
			return err
		}

		done := false
		for !done {
			outFrameSize := sink.FrameSize()

			for fifo.Size() < outFrameSize {
				if done, err = src.Decode(fifo, resampler); err != nil {
					return err
				}
				if done {
					break
				}
			}

			for fifo.Size() >= outFrameSize {
				if err = sink.Encode(fifo); err != nil {
					return err
				}
			}
			if err = writeBuffer(); err != nil {
				return err
			}
		}
		return nil
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Connection", "close")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Cache-Control", "no-cache, no-store")

	for _, path := range os.Args[1:] {
		if err := playFile(path); err != nil {
			log.Printf("failed to play '%v': %v\n", path, err)
		}
	}

	if err = sink.Flush(fifo); err != nil {
		log.Printf("failed to flush sink: %v\n", err)
	}
	if err = sink.WriteTrailer(); err != nil {
		log.Printf("failed to write trailer: %v\n", err)
	}
	if err = writeBuffer(); err != nil {
		log.Printf("failed to write buffer: %v\n", err)
	}
}
