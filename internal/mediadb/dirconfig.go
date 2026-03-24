package mediadb

import (
	"fmt"
	"os"
	"time"

	"go.yaml.in/yaml/v4"
)

const dirConfigName = "aurelius.yaml"

// FragmentConfig defines a single fragment in the directory configuration.
type FragmentConfig struct {
	Source string
	Start  time.Duration
	End    time.Duration
	Artist string
	Title  string
	Album  string
	Track  string
}

// DirConfig represents the contents of an aurelius.yaml file.
type DirConfig struct {
	Fragments []FragmentConfig
}

// rawFragmentConfig is the YAML representation of a FragmentConfig.
type rawFragmentConfig struct {
	Source string `yaml:"source"`
	Start  string `yaml:"start,omitempty"`
	End    string `yaml:"end,omitempty"`
	Artist string `yaml:"artist,omitempty"`
	Title  string `yaml:"title,omitempty"`
	Album  string `yaml:"album,omitempty"`
	Track  string `yaml:"track,omitempty"`
}

// rawDirConfig is the YAML representation of a DirConfig.
type rawDirConfig struct {
	Fragments []rawFragmentConfig `yaml:"fragments,omitempty"`
}

// LoadDirConfig reads and parses an aurelius.yaml file from the given path.
func LoadDirConfig(fsPath string) (*DirConfig, error) {
	f, err := os.Open(fsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw rawDirConfig
	if err := yaml.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", fsPath, err)
	}

	config := &DirConfig{
		Fragments: make([]FragmentConfig, len(raw.Fragments)),
	}
	for i, rf := range raw.Fragments {
		fc := FragmentConfig{
			Source: rf.Source,
			Artist: rf.Artist,
			Title:  rf.Title,
			Album:  rf.Album,
			Track:  rf.Track,
		}
		if rf.Start != "" {
			fc.Start, err = time.ParseDuration(rf.Start)
			if err != nil {
				return nil, fmt.Errorf("fragment %d: invalid start %q: %w", i, rf.Start, err)
			}
		}
		if rf.End != "" {
			fc.End, err = time.ParseDuration(rf.End)
			if err != nil {
				return nil, fmt.Errorf("fragment %d: invalid end %q: %w", i, rf.End, err)
			}
		}
		config.Fragments[i] = fc
	}
	return config, nil
}

// MakeFragmentName creates a synthetic fragment name from a source filename
// and a 1-based fragment index.
func MakeFragmentName(sourceFile string, index int) string {
	return fmt.Sprintf("%s::%03d", sourceFile, index)
}
