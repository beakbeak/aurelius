package media

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (ml *Library) toFileSystemPath(libraryPath string) string {
	return filepath.Join(ml.config.RootPath, libraryPath)
}

func (ml *Library) toLibraryPath(fsPath string) (string, error) {
	libraryPath, err := filepath.Rel(ml.config.RootPath, fsPath)
	if err != nil {
		return "", err
	}

	libraryPath = filepath.ToSlash(libraryPath)

	if strings.HasPrefix(libraryPath, "..") || strings.HasPrefix(libraryPath, "/") {
		return "", fmt.Errorf("path is not under media library root")
	}
	return libraryPath, nil
}

func (ml *Library) toLibraryPathWithContext(fsPath, context string) (string, error) {
	realFsPath, err := filepath.EvalSymlinks(fsPath)
	if err != nil {
		return "", err
	}

	realContext, err := filepath.EvalSymlinks(context)
	if err != nil {
		return "", err
	}

	fsPathInContext, err := filepath.Rel(realContext, realFsPath)
	if err != nil {
		return "", err
	}
	fsPathInContext = filepath.Join(context, fsPathInContext)

	libraryPathInContext, err := ml.toLibraryPath(fsPathInContext)
	if err == nil {
		// contextualized path may be invalid if resolved path is at a
		// different level of indirection than context
		if _, err := os.Stat(ml.toFileSystemPath(libraryPathInContext)); err == nil {
			return libraryPathInContext, nil
		}
	}
	return ml.toLibraryPath(fsPath)
}

func (ml *Library) toUrlPath(libraryPath string) string {
	urlPath := path.Join(ml.config.Prefix, libraryPath)
	return (&url.URL{Path: urlPath}).String()
}

func (ml *Library) toHtmlPath(path string) string {
	return filepath.Join(ml.config.HtmlPath, path)
}
