package main

// TODO:
// - REST(?) API
//   - support embedded images
// - CLI client
// - implement pause (save current source, set it to SilenceSource; restore saved on unpause)
// - server configuration
//   - make ReplayGain mode configurable
//   - basic playlist management
// - seeking
// - combine artists/etc. with different capitalizations/whitespace/accents?/brackets/etc.

// WISHLIST:
// - replaygain preamp?
// - treat sections of a file as playlist entries (e.g. pieces of a long live set, a hidden track)
//   - can't use m3u anymore
// - tag editing
// - get replaygain info from RVA2 mp3 tag (requires another library dependency)
// - figure out why timing is wrong when using MKV container
//   (maybe something to do with time base settings/conversion)
// - direct audio output

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sb/aurelius/aurelib"
)

// TODO: make configurable
const maxBufferedFrames = 32

var debug *log.Logger
var noise *log.Logger
var player *Player

func main() {
	if len(os.Args) < 2 {
		panic("not enough arguments")
	}

	debug = log.New(os.Stdout, "DEBUG: ", log.Ltime|log.Lmicroseconds|log.Ldate|log.Lshortfile)
	noise = log.New(os.Stdout, "NOISE: ", log.Ltime|log.Lmicroseconds|log.Ldate|log.Lshortfile)

	noise.SetOutput(ioutil.Discard)
	noise.SetFlags(0)

	aurelib.Init()

	player = NewPlayer()
	defer player.Destroy()

	player.Play(NewFilePlaylist(os.Args[1:]))

	debug.Println("waiting for connections")
	http.HandleFunc("/", stream)
	log.Fatal(http.ListenAndServe(":9090", nil))
}

type FilePlaylist struct {
	paths []string
	index int
}

func NewFilePlaylist(paths []string) *FilePlaylist {
	return &FilePlaylist{paths: paths, index: -1}
}

func (p *FilePlaylist) get() aurelib.Source {
	var src *aurelib.FileSource
	var err error
	if src, err = aurelib.NewFileSource(p.paths[p.index]); err != nil {
		log.Printf("failed to open '%v': %v", p.paths[p.index], err)
		return nil
	}
	src.DumpFormat()
	debug.Println(src.Tags())
	return src
}

func (p *FilePlaylist) Previous() aurelib.Source {
	for p.index > 0 {
		p.index--
		if src := p.get(); src != nil {
			return src
		}
	}
	return nil
}

func (p *FilePlaylist) Next() aurelib.Source {
	for p.index < (len(p.paths) - 1) {
		p.index++
		if src := p.get(); src != nil {
			return src
		}
	}
	return nil
}

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

	/*
		// flush sink
		for {
			frame, err := fifo.ReadFrame(sink)
			if err != nil {
				log.Printf("failed to flush sink: %v\n", err)
				break
			}
			done := false
			if done, err = sink.Encode(frame); err != nil {
				log.Printf("failed to flush sink: %v\n", err)
				break
			}
			if done {
				break
			}
		}
		if err = sink.WriteTrailer(); err != nil {
			log.Printf("failed to write trailer: %v\n", err)
		}
		if err = writeBuffer(); err != nil {
			log.Printf("failed to write buffer: %v\n", err)
		}
	*/
}
