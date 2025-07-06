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
//	start 15s
//	end 5m30s
package fragment

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/beakbeak/aurelius/internal/maputil"
	"github.com/beakbeak/aurelius/pkg/aurelib"
)

var (
	reFragment = regexp.MustCompile(`(?i)^(.+?)\.([0-9]+)\.txt$`)
	reField    = regexp.MustCompile(`^\s*([^\s]+)\s+(.+?)\s*$`)
	reBlank    = regexp.MustCompile(`^\s*$`)
)

// A Fragment is an aurelib.Source that streams a subsection of an audio file.
type Fragment struct {
	*aurelib.FileSource

	startTime time.Duration
	endTime   time.Duration
	tags      map[string]string
}

// IsFragment returns true if the path is in the format used by fragment
// descriptors.
func IsFragment(path string) bool {
	return reFragment.MatchString(path)
}

// GetSourceFile returns the source audio file path for a fragment file.
// Returns empty string if the path is not a fragment file.
func GetSourceFile(path string) string {
	matches := reFragment.FindStringSubmatch(path)
	if matches == nil {
		return ""
	}
	return matches[1]
}

// New creates a new Fragment from the descriptor specified by path.
func New(path string) (*Fragment, error) {
	f := Fragment{startTime: -1, endTime: -1}
	success := false

	matches := reFragment.FindStringSubmatch(path)
	if matches == nil {
		return &f, fmt.Errorf("invalid filename: %v", filepath.Base(path))
	}
	fragmentIndex := matches[2]

	src, err := aurelib.NewFileSource(matches[1])
	if err != nil {
		return &f, fmt.Errorf("failed to open '%v': %v", matches[1], err)
	}
	defer func() {
		if !success {
			src.Destroy()
		}
	}()

	f.tags = maputil.LowerCaseKeys(src.Tags())
	f.tags["track"] = fmt.Sprintf("%v.%v", f.tags["track"], fragmentIndex)

	if err := f.parse(path); err != nil {
		return &f, err
	}

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
	for scanner.Scan() {
		line := scanner.Text()
		if reBlank.MatchString(line) {
			continue
		}
		matches := reField.FindStringSubmatch(line)
		if matches == nil {
			return fmt.Errorf("invalid line format: %v", line)
		}
		field := matches[1]
		value := matches[2]

		switch field {
		case "start", "end":
			var offset time.Duration
			if offset, err = time.ParseDuration(value); err != nil {
				return fmt.Errorf("invalid value for '%v': %v (%v)", field, value, err)
			}
			if offset < 0 {
				return fmt.Errorf("invalid value for '%v': %v", field, value)
			}
			if field == "start" {
				f.startTime = offset
			} else {
				f.endTime = offset
			}
		case "artist", "title":
			f.tags[field] = value
		default:
			return fmt.Errorf("unknown field: %v", field)
		}
	}
	return scanner.Err()
}

// See aurelib.Source.Tags.
func (f *Fragment) Tags() map[string]string {
	return f.tags
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
