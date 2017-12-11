package database

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sb/aurelius/aurelib"
	"sb/aurelius/util"
	"strconv"
	"time"
)

func (db *Database) handleTrackRequest(
	w http.ResponseWriter,
	req *http.Request,
) (handled bool, _ error) {
	groups := db.reTrackPath.FindStringSubmatch(req.URL.Path)
	if groups == nil {
		return false, nil
	}

	path := db.expandPath(groups[1])
	subRequest := groups[2]

	{
		info, err := os.Stat(path)
		if err != nil {
			return false, nil
		}

		mode := info.Mode()
		if mode.IsDir() {
			return false, nil
		}
		if !mode.IsRegular() {
			return false, fmt.Errorf("not a symlink or regular file: %v", path)
		}
	}

	switch subRequest {
	case "stream":
		util.Noise.Printf("stream %v\n", path)
		db.Stream(path, w, req)
	case "info":
		util.Noise.Printf("info %v\n", path)
		db.Info(path, w, req)
	default:
		return false, fmt.Errorf("invalid DB request: %v", subRequest)
	}
	return true, nil
}

func (db *Database) Info(
	path string,
	w http.ResponseWriter,
	req *http.Request,
) {
	src, err := aurelib.NewFileSource(path)
	if err != nil {
		util.Debug.Printf("failed to open source: %v\n", path)
		http.NotFound(w, req)
		return
	}
	defer src.Destroy()

	type Result struct {
		Name            string            `json:"name"`
		Duration        float64           `json:"duration"`
		ReplayGainTrack float64           `json:"replayGainTrack"`
		ReplayGainAlbum float64           `json:"replayGainAlbum"`
		Tags            map[string]string `json:"tags"`
	}

	result := Result{
		Name:            filepath.Base(path),
		Duration:        float64(src.Duration()) / float64(time.Second),
		ReplayGainTrack: src.ReplayGain(aurelib.ReplayGainTrack, true),
		ReplayGainAlbum: src.ReplayGain(aurelib.ReplayGainAlbum, true),
		Tags:            util.LowerCaseKeys(src.Tags()),
	}
	resultJson, err := json.Marshal(result)
	if err != nil {
		util.Debug.Printf("failed to marshal info JSON: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(resultJson); err != nil {
		util.Debug.Printf("failed to write info response: %v\n", err)
	}
}

func (db *Database) Stream(
	path string,
	w http.ResponseWriter,
	req *http.Request,
) {
	reject := func(status int, format string, args ...interface{}) {
		w.WriteHeader(status)
		util.Debug.Printf(format, args...)
	}
	internalError := func(format string, args ...interface{}) {
		reject(http.StatusInternalServerError, format, args...)
	}
	badRequest := func(format string, args ...interface{}) {
		reject(http.StatusBadRequest, format, args...)
	}
	notFound := func(format string, args ...interface{}) {
		reject(http.StatusNotFound, format, args...)
	}

	// set up source
	src, err := aurelib.NewFileSource(path)
	if err != nil {
		notFound("failed to open '%v': %v\n", path, err)
		return
	}
	defer src.Destroy()

	srcStreamInfo := src.StreamInfo()
	if util.DebugEnabled {
		src.DumpFormat()
	}

	// set up sink
	options := aurelib.NewSinkOptions()
	options.ChannelLayout = srcStreamInfo.ChannelLayout()
	options.SampleFormat = srcStreamInfo.SampleFormat()
	options.SampleRate = srcStreamInfo.SampleRate

	options.Codec = "flac"
	formatName := "flac"
	mimeType := "audio/flac"

	query := req.URL.Query()

	if codec, ok := query["codec"]; ok {
		switch codec[0] {
		case "mp3":
			options.Codec = "libmp3lame"
			formatName = "mp3"
			mimeType = "audio/mp3"
		case "vorbis":
			options.Codec = "libvorbis"
			formatName = "ogg"
			mimeType = "audio/ogg"
		case "flac":
			// already set up
		default:
			badRequest("unknown codec requested: %v\n", codec[0])
			return
		}
	}

	if qualityArgs, ok := query["quality"]; ok {
		if quality, err := strconv.ParseFloat(qualityArgs[0], 32); err == nil {
			options.Quality = float32(quality)
		} else {
			badRequest("invalid quality requested: %v\n", qualityArgs[0])
			return
		}
	}

	if bitRateArgs, ok := query["bitRate"]; ok {
		if bitRate, err := strconv.ParseUint(bitRateArgs[0], 0, 0); err == nil {
			options.BitRate = uint(bitRate) * 1000
		} else {
			badRequest("invalid bit rate requested: %v\n", bitRateArgs[0])
			return
		}
	}

	if sampleRateArgs, ok := query["sampleRate"]; ok {
		if sampleRate, err := strconv.ParseUint(sampleRateArgs[0], 0, 0); err == nil {
			options.SampleRate = uint(sampleRate)
		} else {
			badRequest("invalid sample rate requested: %v\n", sampleRateArgs[0])
			return
		}
	}

	if sampleFormatArgs, ok := query["sampleFormat"]; ok {
		options.SampleFormat = sampleFormatArgs[0]
	}
	if channelLayoutArgs, ok := query["channelLayout"]; ok {
		options.ChannelLayout = channelLayoutArgs[0]
	}

	sink, err := aurelib.NewBufferSink(formatName, options)
	if err != nil {
		internalError("failed to create sink: %v\n", err)
		return
	}
	defer sink.Destroy()

	sinkStreamInfo := sink.StreamInfo()

	// set up FIFO
	fifo, err := aurelib.NewFifo(sinkStreamInfo)
	if err != nil {
		internalError("failed to create FIFO: %v\n", err)
		return
	}
	defer fifo.Destroy()

	// set up resampler
	resampler, err := aurelib.NewResampler()
	if err != nil {
		internalError("failed to create resampler: %v\n", err)
		return
	}
	defer resampler.Destroy()

	if err := resampler.Setup(srcStreamInfo, sinkStreamInfo, 1); err != nil {
		internalError("failed to setup resampler: %v\n", err)
		return
	}

	// start streaming
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "no-cache, no-store") //?

	writeBuffer := func() error {
		buffer := sink.Buffer()
		if len(buffer) == 0 {
			return nil
		}

		count, err := w.Write(buffer)
		if count > 0 {
			sink.Drain(uint(count))
		}
		util.Noise.Printf("wrote %v bytes\n", count)
		return err
	}

	startTime := time.Now()
	playedSamples := uint64(0)

	done := false

PlayLoop:
	for !done {
		fifoSize := fifo.Size()

	DecodeLoop:
		for fifo.Size() < sink.FrameSize() {
			if err, recoverable := src.Decode(); err != nil {
				util.Debug.Printf("failed to decode frame: %v\n", err)
				if !recoverable {
					done = true
					break DecodeLoop
				}
			}

			for {
				receiveStatus, err := src.ReceiveFrame()
				if err != nil {
					util.Debug.Printf("failed to receive frame: %v\n", err)
					done = true
					break DecodeLoop
				}
				if receiveStatus == aurelib.ReceiveFrameEof {
					done = true
					break DecodeLoop
				}
				if receiveStatus != aurelib.ReceiveFrameCopyAndCallAgain {
					break
				}
				if err = src.CopyFrame(fifo, resampler); err != nil {
					util.Debug.Printf("failed to copy frame to output: %v\n", err)
					done = true
					break DecodeLoop
				}
			}
		}

		playedSamples += uint64(fifo.Size() - fifoSize)

		var outFrameSize uint
		if !done {
			outFrameSize = sink.FrameSize()
		} else {
			outFrameSize = 1
		}

		for fifo.Size() >= outFrameSize {
			frame, err := fifo.ReadFrame(sink.FrameSize())
			if err != nil {
				util.Debug.Printf("failed to read frame from FIFO: %v\n", err)
				break PlayLoop
			}
			if _, err = sink.Encode(frame); err != nil {
				util.Debug.Printf("failed to encode frame: %v\n", err)
				break PlayLoop
			}
		}
		if err = writeBuffer(); err != nil {
			util.Debug.Printf("failed to write buffer: %v\n", err)
			break PlayLoop
		}

		// calculate playedTime with millisecond precision to prevent overflow
		playedTime := time.Duration(((playedSamples * 1000) /
			uint64(sinkStreamInfo.SampleRate)) * 1000000)
		timeToSleep := playedTime - playAhead - time.Since(startTime)
		if timeToSleep > time.Millisecond {
			util.Noise.Printf("sleeping %v", timeToSleep)
			time.Sleep(timeToSleep)
		}
	}

	if err = aurelib.FlushSink(sink); err != nil {
		util.Debug.Printf("failed to flush sink: %v\n", err)
	}
	if err = writeBuffer(); err != nil {
		util.Debug.Printf("failed to write buffer: %v\n", err)
	}
}
