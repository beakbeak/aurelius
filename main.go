package main

// TODO:
// - HTTP streaming
//   - need to prevent over-buffering or sending too much data before
//     it can be played
//     - for now: fixed-size buffer; rely on client to drain at appropriate speed
//     - later: throttle encoding speed based on playback speed
//              (available in packet after av_read_frame())
//       - *** this will be necessary for silence when paused ***
// - figure out why timing is wrong when using MKV container
//   (maybe something to do with time base settings/conversion)
// - support embedded images

// WISHLIST:
// - replaygain preamp?
// - treat sections of a file as playlist entries (e.g. pieces of a long live set, a hidden track)
//   - can't use m3u anymore
// - tag editing

import (
	"aurelib"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		panic("not enough arguments")
	}

	aurelib.Init()

	var resampler *aurelib.Resampler
	var err error
	if resampler, err = aurelib.NewResampler(); err != nil {
		panic(err)
	}
	defer resampler.Destroy()

	options := aurelib.NewSinkOptions()

	var sink *aurelib.Sink
	if sink, err = aurelib.NewFileSink(os.Args[len(os.Args)-1], options); err != nil {
		panic(err)
	}
	defer sink.Destroy()

	var fifo *aurelib.Fifo
	if fifo, err = aurelib.NewFifo(sink); err != nil {
		panic(err)
	}
	defer fifo.Destroy()

	playFile := func(path string) error {
		var src *aurelib.Source
		var err error
		if src, err = aurelib.NewFileSource(path); err != nil {
			return err
		}
		defer src.Destroy()

		src.DumpFormat()
		fmt.Println(src.Tags)

		if err := resampler.Setup(
			src, sink, src.ReplayGain(aurelib.ReplayGainTrack, true),
		); err != nil {
			return err
		}

		done := false
		for !done {
			outFrameSize := sink.FrameSize()

			for fifo.Size() < outFrameSize {
				if done, err = src.DecodeFrames(fifo, resampler); err != nil {
					return err
				}
				if done {
					break
				}
			}

			for fifo.Size() >= outFrameSize {
				if err = sink.EncodeFrames(fifo); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, path := range os.Args[1 : len(os.Args)-1] {
		if err := playFile(path); err != nil {
			fmt.Printf("failed to play '%v': %v\n", path, err)
		}
	}

	if err = sink.Flush(fifo); err != nil {
		fmt.Printf("failed to flush sink: %v\n", err)
	}
	if err = sink.WriteTrailer(); err != nil {
		fmt.Printf("failed to write trailer: %v\n", err)
	}
}
