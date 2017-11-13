package main

import (
	"aurelib"
	"fmt"
	"log"
	"time"
)

const maxBufferedCommands = 256

type PlaylistIterator interface {
	Next() aurelib.Source
	Previous() aurelib.Source
}

type Player struct {
	playing  bool
	outputs  map[aurelib.Sink]*playerOutput
	commands chan playerCommandWrapper
}

type playerOutput struct {
	sink      aurelib.Sink
	fifo      *aurelib.Fifo
	resampler *aurelib.Resampler

	frames chan aurelib.Frame
}

type playerCommand interface{}

type playerCommandWrapper struct {
	command playerCommand
	done    chan error
}

func NewPlayer() *Player {
	p := Player{}
	p.outputs = make(map[aurelib.Sink]*playerOutput)
	p.commands = make(chan playerCommandWrapper, maxBufferedCommands)
	return &p
}

type playerCommandShutDown struct{}

func (p *Player) Destroy() {
	if p.playing {
		<-p.sendCommand(playerCommandShutDown{})
	}
	for output := range p.outputs {
		output.Destroy()
	}
}

func (p *Player) sendCommand(command playerCommand) chan error {
	done := make(chan error, 1)
	if !p.playing {
		done <- fmt.Errorf("playback routine is not running")
		return done
	}
	p.commands <- playerCommandWrapper{command, done}
	return done
}

func (o *playerOutput) Destroy() {
	if o.fifo != nil {
		o.fifo.Destroy()
		o.fifo = nil
	}
	if o.resampler != nil {
		o.resampler.Destroy()
		o.resampler = nil
	}

	close(o.frames)
	for frame := range o.frames {
		frame.Destroy()
	}
	o.frames = nil
}

func (p *Player) addOutputImpl(output *playerOutput) error {
	if _, exists := p.outputs[output.sink]; exists {
		return fmt.Errorf("output with provided sink already exists")
	}
	p.outputs[output.sink] = output
	return nil
}

type playerCommandAddOutput struct {
	output *playerOutput
}

// may be called before Play()
// does not take ownership of Sink - it is used only to determine
// sample rate, channel layout and frame size
// (XXX if Sink is destroyed before output is removed or player is shut
//  down, the program will probably crash when Sink attributes are
//  fetched)
func (p *Player) AddOutput(sink aurelib.Sink) (chan aurelib.Frame, error) {
	output := playerOutput{sink: sink}
	success := false
	defer func() {
		if !success {
			output.Destroy()
		}
	}()

	var err error
	if output.fifo, err = aurelib.NewFifo(sink); err != nil {
		return nil, fmt.Errorf("failed to create FIFO: %v", err)
	}
	if output.resampler, err = aurelib.NewResampler(); err != nil {
		return nil, fmt.Errorf("failed to create resampler: %v", err)
	}

	output.frames = make(chan aurelib.Frame, maxBufferedFrames)

	if p.playing {
		p.sendCommand(playerCommandAddOutput{output: &output})
	} else {
		p.addOutputImpl(&output)
	}
	success = true
	return output.frames, nil
}

func (p *Player) removeOutputImpl(sink aurelib.Sink) error {
	output, ok := p.outputs[sink]
	if !ok {
		return fmt.Errorf("output does not exist")
	}
	output.Destroy()
	delete(p.outputs, sink)
	return nil
}

type playerCommandRemoveOutput struct {
	sink aurelib.Sink
}

// may be called before Play()
func (p *Player) RemoveOutput(sink aurelib.Sink) chan error {
	if p.playing {
		return p.sendCommand(playerCommandRemoveOutput{sink: sink})
	}
	done := make(chan error, 1)
	done <- p.removeOutputImpl(sink)
	return done
}

type playerCommandPlay struct {
	playlistIter PlaylistIterator
}

func (p *Player) Play(playlistIter PlaylistIterator) chan error {
	wasPlaying := p.playing
	p.playing = true
	command := playerCommandPlay{playlistIter: playlistIter}
	done := p.sendCommand(command)
	if !wasPlaying {
		debug.Println("starting player routine")
		go p.mainLoop()
	}
	return done
}

type playerCommandStop struct{}

func (p *Player) Stop() chan error {
	return p.sendCommand(playerCommandStop{})
}

type playerCommandNext struct{}

func (p *Player) Next() chan error {
	return p.sendCommand(playerCommandNext{})
}

type playerCommandPrevious struct{}

func (p *Player) Previous() chan error {
	return p.sendCommand(playerCommandPrevious{})
}

type playerCommandTogglePause struct{}

func (p *Player) TogglePause() chan error {
	return p.sendCommand(playerCommandTogglePause{})
}

func (p *Player) mainLoop() {
	debug.Println("player routine started")

	var src aurelib.Source
	var playlistIter PlaylistIterator

	startTime := time.Now()
	totalPlayTime := time.Duration(0)
	trackPlayedSamples := uint64(0)

	defer func() {
		if src != nil {
			src.Destroy()
		}
	}()

	trackPlayTime := func() time.Duration {
		if src != nil {
			return (time.Duration(trackPlayedSamples) * time.Second) / time.Duration(src.SampleRate())
		}
		return 0
	}

	destroySource := func() {
		if src == nil {
			return
		}
		totalPlayTime += trackPlayTime()
		trackPlayedSamples = 0

		src.Destroy()
		src = nil
	}

	// destroys output on callback error
	forEachOutput := func(callback func(output *playerOutput) error) {
		for sink, output := range p.outputs {
			if err := callback(output); err != nil {
				output.Destroy()
				delete(p.outputs, sink)
			}
		}
	}

	setupResampler := func(output *playerOutput) error {
		if src != nil {
			if err := output.resampler.Setup(
				src, output.sink, src.ReplayGain(aurelib.ReplayGainTrack, true),
			); err != nil {
				log.Printf("failed to setup resampler: %v\n", err)
				return err
			}
		}
		return nil
	}

	setSource := func(inSource aurelib.Source) {
		if src != nil {
			destroySource()
		}
		src = inSource
		if src != nil {
			forEachOutput(setupResampler)
		}
	}

	executeCommand := func(wrapper playerCommandWrapper) bool {
		var err error
		shutDown := false

		switch command := wrapper.command.(type) {
		case playerCommandAddOutput:
			if err = setupResampler(command.output); err == nil {
				err = p.addOutputImpl(command.output)
			}
			if err != nil {
				command.output.Destroy()
			}
		case playerCommandRemoveOutput:
			err = p.removeOutputImpl(command.sink)
		case playerCommandPlay:
			playlistIter = command.playlistIter
			destroySource()
		case playerCommandStop:
			playlistIter = nil
			destroySource()
		case playerCommandNext:
			if playlistIter != nil {
				destroySource()
			}
		case playerCommandPrevious:
			if playlistIter != nil {
				setSource(playlistIter.Previous())
			}
		case playerCommandTogglePause:
			panic("TODO: pause")
		case playerCommandShutDown:
			shutDown = true
		default:
			log.Printf("unknown player command: %v\n", command)
		}

		wrapper.done <- err
		return shutDown
	}

MainLoop:
	for {
	CommandLoop:
		for {
			select {
			case command := <-p.commands:
				if executeCommand(command) {
					break MainLoop
				}
			default:
				break CommandLoop
			}
		}

		// set up audio source if necessary
		if src == nil {
			if playlistIter != nil {
				setSource(playlistIter.Next())
			}
			if src == nil {
				playlistIter = nil
				panic("TODO: create silence source")
			}
		}

		// decode a frame of audio data
		if err, recoverable := src.Decode(); err != nil {
			log.Printf("failed to decode frame: %v\n", err)
			if !recoverable {
				destroySource()
			}
			continue MainLoop
		}

		var receiveStatus aurelib.ReceiveFrameStatus
		for {
			var err error
			if receiveStatus, err = src.ReceiveFrame(); err != nil {
				log.Printf("failed to receive frame: %v\n", err)
				destroySource()
				continue MainLoop
			}
			if receiveStatus != aurelib.ReceiveFrameCopyAndCallAgain {
				break
			}

			trackPlayedSamples += uint64(src.FrameSize())

			// push frame to each output FIFO and pop frames of the
			// appropriate size into the output frames channel
			forEachOutput(func(output *playerOutput) error {
				if err = src.CopyFrame(output.fifo, output.resampler); err != nil {
					log.Printf("failed to copy frame to output: %v\n", err)
					return err
				}

				for output.fifo.Size() >= output.sink.FrameSize() {
					var frame aurelib.Frame
					if frame, err = output.fifo.ReadFrame(output.sink); err != nil {
						log.Printf("failed to read frame from FIFO: %v\n", err)
						return err
					}
					select {
					case output.frames <- frame: // try to send frame
					default: // discard if channel is full
					}
				}
				return nil
			})
		}
		if receiveStatus == aurelib.ReceiveFrameEof {
			destroySource()
		}

		// sleep according to playback speed
		playTime := totalPlayTime + trackPlayTime()
		timeToSleep := playTime - time.Since(startTime)
		if timeToSleep > time.Millisecond {
			noise.Printf("sleeping %v\n", timeToSleep)
			time.Sleep(timeToSleep)
		}
	}
}
