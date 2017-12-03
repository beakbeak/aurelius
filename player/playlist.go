package player

import (
	"log"
	"sb/aurelius/aurelib"
	"sb/aurelius/aurelog"
)

type FilePlaylist struct {
	paths []string
	index int
}

func NewFilePlaylist(paths []string) *FilePlaylist {
	return &FilePlaylist{paths: paths, index: -1}
}

func (p *FilePlaylist) get() aurelib.Source {
	src, err := aurelib.NewFileSource(p.paths[p.index])
	if err != nil {
		log.Printf("failed to open '%v': %v", p.paths[p.index], err)
		return nil
	}
	if aurelog.DebugEnabled {
		src.DumpFormat()
		aurelog.Debug.Println(src.Tags())
	}
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