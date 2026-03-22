// Package fragment provides an aurelib.Source that streams a subsection of an
// audio file.
package fragment

import (
	"fmt"
	"time"

	"github.com/beakbeak/aurelius/pkg/aurelib"
)

// A Fragment is an aurelib.Source that streams a subsection of an audio file.
type Fragment struct {
	*aurelib.FileSource

	startTime time.Duration
	endTime   time.Duration
}

// New creates a new Fragment from a source audio file path and start/end
// times. A zero endTime means the end of the file.
func New(sourceFilePath string, startTime, endTime time.Duration) (*Fragment, error) {
	f := Fragment{startTime: startTime, endTime: endTime}
	success := false

	src, err := aurelib.NewFileSource(sourceFilePath)
	if err != nil {
		return &f, fmt.Errorf("failed to open '%v': %w", sourceFilePath, err)
	}
	defer func() {
		if !success {
			src.Destroy()
		}
	}()

	duration := src.Duration()

	if f.startTime < 0 {
		f.startTime = 0
	} else if f.startTime > duration {
		f.startTime = duration
	}

	if f.endTime <= 0 {
		f.endTime = duration
	} else if f.endTime > duration {
		f.endTime = duration
	}

	if f.startTime >= f.endTime {
		return &f, fmt.Errorf("start >= end (%v >= %v)", f.startTime, f.endTime)
	}

	if f.startTime > 0 {
		if err := src.SeekTo(f.startTime); err != nil {
			return &f, fmt.Errorf("seek failed: %w", err)
		}
	}

	f.FileSource = src
	success = true
	return &f, nil
}

// See aurelib.Source.Duration.
func (f *Fragment) Duration() time.Duration {
	return f.endTime - f.startTime
}

// See aurelib.Source.SeekTo.
func (f *Fragment) SeekTo(offset time.Duration) error {
	offset += f.startTime
	if offset > f.endTime {
		offset = f.endTime
	}
	return f.FileSource.SeekTo(offset)
}

// See aurelib.Source.ReceiveFrame.
func (f *Fragment) ReceiveFrame() (aurelib.ReceiveFrameStatus, error) {
	status, err := f.FileSource.ReceiveFrame()
	if err == nil && status == aurelib.ReceiveFrameCopyAndCallAgain && f.FileSource.FrameStartTime() >= f.endTime {
		return aurelib.ReceiveFrameEof, nil
	}
	return status, err
}
