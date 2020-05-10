package database

import (
	"net/http"
	"sb/aurelius/aurelib"
	"strconv"
	"time"
)

// SetThrottleStreaming controls whether streaming throughput is limited to
// playback speed. If set to false, streaming throughput is not limited.
// Default: true.
func (db *Database) SetThrottleStreaming(value bool) {
	db.throttleStreaming = value
}

// ThrottleStreaming returns whether streaming throughput is limited to playback
// speed. If false, streaming throughput is not limited. Default: true.
func (db *Database) ThrottleStreaming() bool {
	return db.throttleStreaming
}

// SetDeterministicStreaming controls whether to avoid randomness in encoding
// and muxing. It should be enabled when deterministic output is needed, such as
// when performing automated testing. Default: false.
func (db *Database) SetDeterministicStreaming(value bool) {
	db.deterministicStreaming = value
}

// DeterministicStreaming returns whether to avoid randomness in encoding and
// muxing. It should be enabled when deterministic output is needed, such as
// when performing automated testing. Default: false.
func (db *Database) DeterministicStreaming() bool {
	return db.deterministicStreaming
}

func (db *Database) handleStreamRequest(
	path string,
	w http.ResponseWriter,
	req *http.Request,
) {
	reject := func(status int, format string, args ...interface{}) {
		w.WriteHeader(status)
		logger(LogDebug).Printf(format, args...)
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
	src, err := newAudioSource(path)
	if err != nil {
		notFound("failed to open '%v': %v\n", path, err)
		return
	}
	defer src.Destroy()

	srcStreamInfo := src.StreamInfo()
	/*
		if logLevel >= LogDebug {
			src.DumpFormat()
		}
	*/

	// set up sink
	options := aurelib.NewSinkOptions()
	options.ChannelLayout = srcStreamInfo.ChannelLayout()
	options.SampleFormat = srcStreamInfo.SampleFormat()
	options.SampleRate = srcStreamInfo.SampleRate

	options.Codec = "pcm_s16le"
	formatName := "wav"
	mimeType := "audio/wav"
	replayGainStr := "track"
	preventClipping := true

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
			options.Codec = "flac"
			formatName = "flac"
			mimeType = "audio/flac"
		case "wav":
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
			badRequest("invalid quality requested: %v (%v)\n", qualityArgs[0], err)
			return
		}
	}

	if bitRateArgs, ok := query["bitRate"]; ok {
		if bitRate, err := strconv.ParseUint(bitRateArgs[0], 0, 0); err == nil {
			options.BitRate = uint(bitRate) * 1000
		} else {
			badRequest("invalid bit rate requested: %v (%v)\n", bitRateArgs[0], err)
			return
		}
	}

	if sampleRateArgs, ok := query["sampleRate"]; ok {
		if sampleRate, err := strconv.ParseUint(sampleRateArgs[0], 0, 0); err == nil {
			options.SampleRate = uint(sampleRate)
		} else {
			badRequest("invalid sample rate requested: %v (%v)\n", sampleRateArgs[0], err)
			return
		}
	}

	if sampleFormatArgs, ok := query["sampleFormat"]; ok {
		options.SampleFormat = sampleFormatArgs[0]
	}
	if channelLayoutArgs, ok := query["channelLayout"]; ok {
		options.ChannelLayout = channelLayoutArgs[0]
	}
	if replayGainArgs, ok := query["replayGain"]; ok {
		replayGainStr = replayGainArgs[0]
	}

	if preventClippingArgs, ok := query["preventClipping"]; ok {
		if preventClipping, err = strconv.ParseBool(preventClippingArgs[0]); err != nil {
			badRequest("invalid value for preventClipping: %v (%v)\n", preventClippingArgs[0], err)
			return
		}
	}

	volume := 1.
	switch replayGainStr {
	case "track":
		volume = src.ReplayGain(aurelib.ReplayGainTrack, preventClipping)
	case "album":
		volume = src.ReplayGain(aurelib.ReplayGainAlbum, preventClipping)
	case "off":
		// already set up
	default:
		badRequest("invalid ReplayGain mode: %v\n", replayGainStr)
		return
	}

	// volume < 1 is applied on the client side for better quality
	// volume > 1 is applied on the server side due to limitations in the HTML5 media API
	if volume < 1 {
		volume = 1.
	}

	if startTimeArgs, ok := query["startTime"]; ok {
		startTime, err := time.ParseDuration(startTimeArgs[0])
		if err != nil {
			badRequest("invalid start time: %v (%v)\n", startTimeArgs[0], err)
			return
		} else if startTime < 0 {
			badRequest("invalid start time: %v\n", startTimeArgs[0])
			return
		} else if startTime > 0 {
			if err := src.SeekTo(startTime); err != nil {
				logger(LogDebug).Printf("seek failed: %v\n", err)
			}
		}
	}

	if db.deterministicStreaming {
		options.BitExact = true
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

	if err := resampler.Setup(srcStreamInfo, sinkStreamInfo, volume); err != nil {
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
		logger(LogNoise).Printf("wrote %v bytes\n", count)
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
				logger(LogDebug).Printf("failed to decode frame: %v\n", err)
				if !recoverable {
					done = true
					break DecodeLoop
				}
			}

			for {
				receiveStatus, err := src.ReceiveFrame()
				if err != nil {
					logger(LogDebug).Printf("failed to receive frame: %v\n", err)
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
				if err = src.ResampleFrame(resampler, fifo); err != nil {
					logger(LogDebug).Printf("failed to copy frame to output: %v\n", err)
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
				logger(LogDebug).Printf("failed to read frame from FIFO: %v\n", err)
				break PlayLoop
			}
			if _, err = sink.Encode(frame); err != nil {
				logger(LogDebug).Printf("failed to encode frame: %v\n", err)
				break PlayLoop
			}
		}
		if err = writeBuffer(); err != nil {
			logger(LogDebug).Printf("failed to write buffer: %v\n", err)
			break PlayLoop
		}

		if db.throttleStreaming {
			// calculate playedTime with millisecond precision to prevent overflow
			playedTime := time.Duration(((playedSamples * 1000) /
				uint64(sinkStreamInfo.SampleRate)) * 1000000)
			timeToSleep := playedTime - playAhead - time.Since(startTime)
			if timeToSleep > time.Millisecond {
				logger(LogNoise).Printf("sleeping %v", timeToSleep)
				time.Sleep(timeToSleep)
			}
		}
	}

	if err = aurelib.FlushSink(sink); err != nil {
		logger(LogDebug).Printf("failed to flush sink: %v\n", err)
	}
	if err = writeBuffer(); err != nil {
		logger(LogDebug).Printf("failed to write buffer: %v\n", err)
	}
}
