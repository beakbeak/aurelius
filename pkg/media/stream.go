package media

import (
	"net/http"
	"strconv"
	"time"

	"github.com/beakbeak/aurelius/pkg/aurelib"
)

func (ml *Library) handleTrackStreamRequest(
	fsPath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	reject := func(status int, format string, args ...interface{}) {
		w.WriteHeader(status)
		log.Printf(format, args...)
	}
	rejectInternalError := func(format string, args ...interface{}) {
		reject(http.StatusInternalServerError, format, args...)
	}
	rejectBadRequest := func(format string, args ...interface{}) {
		reject(http.StatusBadRequest, format, args...)
	}
	rejectNotFound := func(format string, args ...interface{}) {
		reject(http.StatusNotFound, format, args...)
	}

	// set up source
	src, err := newAudioSource(fsPath)
	if err != nil {
		rejectNotFound("failed to open '%v': %v\n", fsPath, err)
		return
	}
	defer src.Destroy()

	srcStreamInfo := src.StreamInfo()

	// set up sink
	config := aurelib.NewSinkConfig()
	config.ChannelLayout = srcStreamInfo.ChannelLayout()
	config.SampleFormat = srcStreamInfo.SampleFormat()
	config.SampleRate = srcStreamInfo.SampleRate

	replayGainStr := "track"
	preventClipping := true

	query := req.URL.Query()

	codec := "wav"
	if codecArgs, ok := query["codec"]; ok {
		codec = codecArgs[0]
	}

	var formatName, mimeType string

	switch codec {
	case "mp3":
		config.Codec = "libmp3lame"
		formatName = "mp3"
		mimeType = "audio/mp3"

	case "vorbis":
		config.Codec = "libvorbis"
		formatName = "ogg"
		mimeType = "audio/ogg"

	case "flac":
		config.Codec = "flac"

		// The FLAC container format seems to be handled poorly by browsers with
		// respect to seeking and reporting which regions are buffered. This may
		// be caused by streaming the data before final timing information has
		// been written by FFmpeg.
		//
		// A FLAC stream inside an Ogg container doesn't seem to have this
		// problem, so we use that instead.
		formatName = "ogg"
		mimeType = "audio/ogg"

	case "wav":
		config.Codec = "pcm_s16le"
		formatName = "wav"
		mimeType = "audio/wav"

	default:
		rejectBadRequest("unknown codec requested: %v\n", codec[0])
		return
	}

	if qualityArgs, ok := query["quality"]; ok {
		if quality, err := strconv.ParseFloat(qualityArgs[0], 32); err == nil {
			config.Quality = float32(quality)
		} else {
			rejectBadRequest("invalid quality requested: %v (%v)\n", qualityArgs[0], err)
			return
		}
	}

	if kbitRateArgs, ok := query["kbitRate"]; ok {
		if kbitRate, err := strconv.ParseUint(kbitRateArgs[0], 0, 0); err == nil {
			config.BitRate = uint(kbitRate) * 1000
		} else {
			rejectBadRequest("invalid kbit rate requested: %v (%v)\n", kbitRateArgs[0], err)
			return
		}
	}

	if sampleRateArgs, ok := query["sampleRate"]; ok {
		if sampleRate, err := strconv.ParseUint(sampleRateArgs[0], 0, 0); err == nil {
			config.SampleRate = uint(sampleRate)
		} else {
			rejectBadRequest("invalid sample rate requested: %v (%v)\n", sampleRateArgs[0], err)
			return
		}
	}

	if sampleFormatArgs, ok := query["sampleFormat"]; ok {
		config.SampleFormat = sampleFormatArgs[0]
	}
	if channelLayoutArgs, ok := query["channelLayout"]; ok {
		config.ChannelLayout = channelLayoutArgs[0]
	}
	if replayGainArgs, ok := query["replayGain"]; ok {
		replayGainStr = replayGainArgs[0]
	}

	if preventClippingArgs, ok := query["preventClipping"]; ok {
		if preventClipping, err = strconv.ParseBool(preventClippingArgs[0]); err != nil {
			rejectBadRequest("invalid value for preventClipping: %v (%v)\n", preventClippingArgs[0], err)
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
		rejectBadRequest("invalid ReplayGain mode: %v\n", replayGainStr)
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
			rejectBadRequest("invalid start time: %v (%v)\n", startTimeArgs[0], err)
			return
		} else if startTime < 0 {
			rejectBadRequest("invalid start time: %v\n", startTimeArgs[0])
			return
		} else if startTime > 0 {
			if err := src.SeekTo(startTime); err != nil {
				log.Printf("seek failed: %v\n", err)
			}
		}
	}

	if ml.config.DeterministicStreaming {
		config.BitExact = true
	}

	sink, err := aurelib.NewBufferSink(formatName, config)
	if err != nil {
		rejectInternalError("failed to create sink: %v\n", err)
		return
	}
	defer sink.Destroy()

	sinkStreamInfo := sink.StreamInfo()

	// set up FIFO
	fifo, err := aurelib.NewFifo(sinkStreamInfo)
	if err != nil {
		rejectInternalError("failed to create FIFO: %v\n", err)
		return
	}
	defer fifo.Destroy()

	// set up resampler
	resampler, err := aurelib.NewResampler()
	if err != nil {
		rejectInternalError("failed to create resampler: %v\n", err)
		return
	}
	defer resampler.Destroy()

	if err := resampler.Setup(srcStreamInfo, sinkStreamInfo, volume); err != nil {
		rejectInternalError("failed to setup resampler: %v\n", err)
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
		//log.Printf("wrote %v bytes\n", count)
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
				log.Printf("failed to decode frame: %v\n", err)
				if !recoverable {
					done = true
					break DecodeLoop
				}
			}

			for {
				receiveStatus, err := src.ReceiveFrame()
				if err != nil {
					log.Printf("failed to receive frame: %v\n", err)
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
					log.Printf("failed to copy frame to output: %v\n", err)
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
				log.Printf("failed to read frame from FIFO: %v\n", err)
				break PlayLoop
			}
			if _, err = sink.Encode(frame); err != nil {
				log.Printf("failed to encode frame: %v\n", err)
				break PlayLoop
			}
		}
		if err = writeBuffer(); err != nil {
			log.Printf("failed to write buffer: %v\n", err)
			break PlayLoop
		}

		if ml.config.ThrottleStreaming && ml.config.PlayAhead > 0 {
			// calculate playedTime with millisecond precision to prevent overflow
			playedTime := time.Duration(((playedSamples * 1000) /
				uint64(sinkStreamInfo.SampleRate)) * 1000000)
			timeToSleep := playedTime - ml.config.PlayAhead - time.Since(startTime)
			if timeToSleep > time.Millisecond {
				//log.Printf("sleeping %v", timeToSleep)
				time.Sleep(timeToSleep)
			}
		}
	}

	if err = aurelib.FlushSink(sink); err != nil {
		log.Printf("failed to flush sink: %v\n", err)
	}
	if err = writeBuffer(); err != nil {
		log.Printf("failed to write buffer: %v\n", err)
	}
}
