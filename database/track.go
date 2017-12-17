package database

import (
	"net/http"
	"os"
	"path/filepath"
	"sb/aurelius/aurelib"
	"sb/aurelius/util"
	"strconv"
	"time"
)

const favoritesPath = "Favorites.m3u"

func (db *Database) handleTrackRequest(
	matches []string,
	w http.ResponseWriter,
	req *http.Request,
) {
	urlPath := matches[1]
	filePath := db.expandPath(urlPath)
	if info, err := os.Stat(filePath); err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, req)
		return
	}

	subRequest := matches[2]
	handled := true

	switch req.Method {
	case http.MethodGet:
		switch subRequest {
		case "stream":
			db.handleStreamRequest(filePath, w, req)
		case "info":
			db.handleInfoRequest(urlPath, filePath, w, req)
		default:
			handled = false
		}
	case http.MethodPost:
		switch subRequest {
		case "favorite":
			if err := db.Favorite(urlPath); err != nil {
				util.Debug.Printf("Favorite failed: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				util.WriteJson(w, nil)
			}
		case "unfavorite":
			if err := db.Unfavorite(urlPath); err != nil {
				util.Debug.Printf("Unfavorite failed: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				util.WriteJson(w, nil)
			}
		default:
			handled = false
		}
	default:
		handled = false
	}

	if !handled {
		http.NotFound(w, req)
	}
}

func (db *Database) IsFavorite(path string) (bool, error) {
	favorites, err := db.playlistCache.Get(db.expandPath(favoritesPath))
	if err != nil {
		return false, err
	}

	for _, line := range favorites {
		if line == path {
			return true, nil
		}
	}
	return false, nil
}

func (db *Database) handleInfoRequest(
	urlPath string,
	filePath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	src, err := aurelib.NewFileSource(filePath)
	if err != nil {
		util.Debug.Printf("failed to open source: %v\n", filePath)
		http.NotFound(w, req)
		return
	}
	defer src.Destroy()

	type Result struct {
		Name            string            `json:"name"`
		Duration        float64           `json:"duration"`
		ReplayGainTrack float64           `json:"replayGainTrack"`
		ReplayGainAlbum float64           `json:"replayGainAlbum"`
		Favorite        bool              `json:"favorite"`
		Tags            map[string]string `json:"tags"`
	}

	result := Result{
		Name:            filepath.Base(filePath),
		Duration:        float64(src.Duration()) / float64(time.Second),
		ReplayGainTrack: src.ReplayGain(aurelib.ReplayGainTrack, true),
		ReplayGainAlbum: src.ReplayGain(aurelib.ReplayGainAlbum, true),
		Tags:            util.LowerCaseKeys(src.Tags()),
	}

	if favorite, err := db.IsFavorite(urlPath); err != nil {
		util.Debug.Printf("isFavorite failed: %v", err)
	} else {
		result.Favorite = favorite
	}

	util.WriteJson(w, result)
}

func (db *Database) Favorite(path string) error {
	return db.playlistCache.Modify(
		db.expandPath(favoritesPath),
		func(favorites []string) ([]string, error) {
			for _, line := range favorites {
				if line == path {
					return nil, nil
				}
			}
			return append(favorites, path), nil
		},
	)
}

func (db *Database) Unfavorite(path string) error {
	return db.playlistCache.Modify(
		db.expandPath(favoritesPath),
		func(favorites []string) ([]string, error) {
			for index, line := range favorites {
				if line == path {
					return append(favorites[:index], favorites[index+1:]...), nil
				}
			}
			return nil, nil
		},
	)
}

func (db *Database) handleStreamRequest(
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
	if replayGainArgs, ok := query["replayGain"]; ok {
		replayGainStr = replayGainArgs[0]
	}

	if preventClippingArgs, ok := query["preventClipping"]; ok {
		if preventClipping, err = strconv.ParseBool(preventClippingArgs[0]); err != nil {
			badRequest("invalid value for preventClipping: %v\n", preventClippingArgs[0])
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
