// Package fragment provides an aurelib.Source that streams a subsection of an
// audio file.
//
// The subsection is defined in a text file with the same name as the source
// audio file with an index number and ".txt" appended. For example,
// MyTrack.flac.1.txt would describe subsection 1 of MyTrack.flac. The index
// number is appended to the source file's "track" tag.
//
// The descriptor file contains lines indicating the start and/or end times of
// the subsection. If either time is unspecified, the start or end time of the
// source file will be used, respectively.
//
// The lines are written as "start" or "end" followed by a time accepted by
// time.ParseDuration. For example:
//
//     start 15s
//     end 5m30s
package fragment

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/beakbeak/aurelius/pkg/aurelib"
)

var (
	reFragment = regexp.MustCompile(`(?i)^(.+?)\.([0-9]+)\.txt$`)
)

// A Fragment is an aurelib.Source that streams a subsection of an audio file.
type Fragment struct {
	*aurelib.FileSource

	startTime time.Duration
	endTime   time.Duration
}

// IsFragment returns true if the path is in the format used by fragment
// descriptors.
func IsFragment(path string) bool {
	return reFragment.MatchString(path)
}

// New creates a new Fragment from the descriptor specified by path.
func New(path string) (*Fragment, error) {
	f := Fragment{startTime: -1, endTime: -1}
	success := false

	matches := reFragment.FindStringSubmatch(path)
	if matches == nil {
		return &f, fmt.Errorf("invalid filename: %v", filepath.Base(path))
	}

	if err := f.parse(path); err != nil {
		return &f, err
	}

	src, err := aurelib.NewFileSource(matches[1])
	if err != nil {
		return &f, fmt.Errorf("failed to open '%v': %v", matches[1], err)
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

	if f.endTime < 0 {
		f.endTime = duration
	} else if f.endTime > duration {
		f.endTime = duration
	}

	if f.startTime >= f.endTime {
		return &f, fmt.Errorf("start >= end (%v >= %v)", f.startTime, f.endTime)
	}

	if f.startTime > 0 {
		if err := src.SeekTo(f.startTime); err != nil {
			return &f, fmt.Errorf("seek failed: %v", err)
		}
	}

	tags := src.Tags()
	tags["track"] = fmt.Sprintf("%v.%v", tags["track"], matches[2])

	f.FileSource = src
	success = true
	return &f, nil
}

func (f *Fragment) parse(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)

StatementLoop:
	for {
		const tokenCount = 2
		var tokens [tokenCount]string
		for i := 0; i < tokenCount; i++ {
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return err
				}
				if i == 0 {
					break StatementLoop
				}
			}
			tokens[i] = scanner.Text()
		}

		var offset time.Duration
		switch tokens[0] {
		case "start":
			fallthrough
		case "end":
			var err error
			if offset, err = time.ParseDuration(tokens[1]); err != nil {
				return fmt.Errorf("invalid value for '%v': %v (%v)", tokens[0], tokens[1], err)
			}
			if offset < 0 {
				return fmt.Errorf("invalid value for '%v': %v", tokens[0], tokens[1])
			}
		default:
			return fmt.Errorf("invalid key: %v", tokens[0])
		}

		switch tokens[0] {
		case "start":
			f.startTime = offset
		case "end":
			f.endTime = offset
		}
	}
	return nil
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
