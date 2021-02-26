// Package dropbox provides an afero implementation to the dropbox API
package dropbox

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
	"github.com/spf13/afero"
)

// Fs is the dropbox filesystem
type Fs struct {
	conf         dropbox.Config
	files        files.Client
	rootPath     string
	dirListLimit int
}

// NewFs creates new dropbox FS instance
func NewFs(token string) *Fs {
	fs := &Fs{}
	fs.conf = dropbox.Config{
		Token:    token,
		LogLevel: dropbox.LogInfo,
	}

	fs.files = files.New(fs.conf)

	return fs
}

// Create creates a file.
// This implementation respects the afero specs but is super slow because it will always open the file two times.
func (fs *Fs) Create(name string) (afero.File, error) {
	f, err := fs.OpenFile(name, os.O_WRONLY, 0777)
	if err != nil {
		return nil, err
	}

	/*
		if _, errWrite := f.WriteString(""); errWrite != nil {
			return nil, fmt.Errorf("couldn't ")
		}
	*/

	// We close it to write the file
	if errClose := f.Close(); errClose != nil {
		return nil, fmt.Errorf("issue while closing empty file: %w", errClose)
	}

	// And re-open it to allow to write on it
	return f, f.(*File).openWriteStream()
}

// Mkdir creates a directory.
func (fs *Fs) Mkdir(name string, _ os.FileMode) error {
	p := path.Join(fs.rootPath, name)

	_, err := fs.files.CreateFolderV2(&files.CreateFolderArg{Path: p})

	return fmt.Errorf("couldn't create dir: %w", err)
}

// MkdirAll creates a directory and all parent directories if necessary.
func (fs *Fs) MkdirAll(name string, perm os.FileMode) error {
	p := path.Join(fs.rootPath, name)

	parts := strings.Split(p, "/")
	totalPath := ""

	for _, part := range parts {
		if part == "" {
			continue
		}

		totalPath += "/" + part
		if _, err := fs.Stat(totalPath); err != nil {
			if os.IsNotExist(err) {
				if err = fs.Mkdir(name, perm); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}

// Open a file for reading.
func (fs *Fs) Open(name string) (afero.File, error) {
	return fs.OpenFile(name, os.O_RDONLY, 0777)
}

// OpenFile opens a file.
func (fs *Fs) OpenFile(name string, flag int, _ os.FileMode) (afero.File, error) {
	p := path.Join(fs.rootPath, name)

	file := newFile(fs, p)

	// Reading and writing is technically supported but can't lead to anything that makes sense
	if flag&os.O_RDWR != 0 {
		return nil, ErrNotSupported
	}

	// Dropbox doesn't support it:
	// https://www.dropboxforum.com/t5/Dropbox-API-Support-Feedback/How-to-append-to-existing-file/td-p/271603
	if flag&os.O_APPEND != 0 {
		return nil, ErrNotSupported
	}

	// Creating is basically a write
	if flag&os.O_CREATE != 0 {
		flag |= os.O_WRONLY
	}

	// We either write
	if flag&os.O_WRONLY != 0 {
		return file, file.openWriteStream()
	}

	info, err := file.Stat()

	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return file, nil
	}

	return file, file.openReadStream(0)
}

// Remove removes a file
func (fs *Fs) Remove(name string) error {
	_, err := fs.files.DeleteV2(&files.DeleteArg{Path: path.Join(fs.rootPath, name)})

	return fmt.Errorf("couldn't remove a file: %w", err)
}

// RemoveAll removes all files inside a directory
func (fs *Fs) RemoveAll(name string) error {
	return fs.Remove(name)
}

// Rename renames a file
func (fs *Fs) Rename(oldname, newname string) error {
	_, err := fs.files.MoveV2(&files.RelocationArg{RelocationPath: files.RelocationPath{
		FromPath: path.Join(fs.rootPath, oldname),
		ToPath:   path.Join(fs.rootPath, newname),
	}})

	return fmt.Errorf("couldn't rename file: %w", err)
}

// Stat fetches the file info
func (fs *Fs) Stat(name string) (os.FileInfo, error) {
	p := path.Join(fs.rootPath, name)

	return fs.stat(p)
}

func (fs *Fs) stat(name string) (os.FileInfo, error) {
	meta, err := fs.files.GetMetadata(&files.GetMetadataArg{Path: name})

	if err != nil {
		var errMetadataAPIError files.GetMetadataAPIError
		if errors.As(err, &errMetadataAPIError) && strings.HasPrefix(errMetadataAPIError.ErrorSummary, "path/not_found/") {
			return nil, os.ErrNotExist
		}

		return nil, fmt.Errorf("couldn't fetch file info: %w", err)
	}

	return newFileInfo(meta), nil
}

// Name of the fs: dropbox
func (fs *Fs) Name() string {
	return "dropbox"
}

// Chmod is not supported
func (fs *Fs) Chmod(name string, mode os.FileMode) error {
	return ErrNotSupported
}

// Chown is not supported
func (fs *Fs) Chown(name string, uid int, gid int) error {
	return ErrNotSupported
}

// Chtimes is not supported because dropbox doesn't support simply changing a time
func (fs *Fs) Chtimes(name string, _ time.Time, mtime time.Time) error {
	return ErrNotSupported
}

// SetRootDirectory defines a base directory
// This is mostly useful to isolate tests and can most probably forgotten
// for most use-cases.
func (fs *Fs) SetRootDirectory(fullPath string) {
	fs.rootPath = fullPath
}
