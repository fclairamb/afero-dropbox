package dropbox

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// nolint: gochecknoglobals
var (
	suffix   string
	initOnce sync.Once
)

func varInit() {
	suffix = time.Now().UTC().Format("20060102_150405.000000")
}

func loadEnvFromFile(t *testing.T) {
	env, err := ioutil.ReadFile(".env.json")
	if err != nil {
		if !os.IsNotExist(err) {
			require.NoError(t, err)
		}
	}

	if len(env) > 0 {
		var environmentVariables map[string]interface{}

		require.NoError(t, json.Unmarshal(env, &environmentVariables))

		for key, val := range environmentVariables {
			if s, ok := val.(string); ok {
				require.NoError(t, os.Setenv(key, s))
			} else {
				require.FailNow(t, "unable to set environment", "Key `%s' is not a string was a %T", key, val)
			}
		}
	}
}

func setup(t *testing.T) (*Fs, *require.Assertions) {
	initOnce.Do(varInit)

	// All of our tests can run in parallel
	// t.Parallel()
	req := require.New(t)

	loadEnvFromFile(t)

	token := os.Getenv("DROPBOX_TOKEN")
	// t.Log("Token: " + token[:4] + "..." + token[len(token)-4:])

	fs := NewFs(token)

	fullPath := "/" + sanitizeName(fmt.Sprintf("Test-%s-%s", t.Name()[4:], suffix))

	err := fs.MkdirAll(fullPath, os.FileMode(700))
	req.NoError(err)

	fs.SetRootDirectory(fullPath)

	t.Cleanup(func() {
		fs.SetRootDirectory("")
		req.NoError(fs.RemoveAll(fullPath))
	})

	return fs, req
}

func sanitizeName(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if isPathSeperator(r) || r == '\'' {
			runes[i] = '-'
		}
	}

	return string(runes)
}

func isPathSeperator(r rune) bool {
	return r == '/' || r == '\\'
}

func TestCompatibility(t *testing.T) {
	var _ afero.Fs = (*Fs)(nil)
	var _ afero.File = (*File)(nil)
}

func TestGetFs(t *testing.T) {
	fs, req := setup(t)
	req.NotNil(fs)
}

func TestMkdir(t *testing.T) {
	fs, req := setup(t)
	req.NoError(fs.Mkdir("dir1", 0))
	info, err := fs.Stat("dir1")
	req.NoError(err)
	req.True(info.IsDir())
}

func TestCreateFile(t *testing.T) {
	fs, req := setup(t)

	f, err := fs.Create("file1")
	req.NoError(err)
	req.NotNil(f)
}

func TestRenameFile(t *testing.T) {
	fs, req := setup(t)

	f, err := fs.Create("file1")
	req.NoError(err)
	req.NotNil(f)

	err = fs.Rename("file1", "file2")
	req.NoError(err)

	s, err := fs.Stat("file1")
	req.EqualError(err, os.ErrNotExist.Error())
	req.Nil(s)

	s, err = fs.Stat("file2")
	req.NoError(err)
	req.False(s.IsDir())
}

func TestStat(t *testing.T) {
	fs, req := setup(t)

	f, err := fs.OpenFile("file1", os.O_WRONLY, 0)
	req.NoError(err)
	req.NotNil(f)

	written, err := f.WriteString("some content")
	req.NoError(err)
	req.Equal(12, written)

	// Closing the file
	req.NoError(f.Close())

	// Using the cache
	info, err := f.Stat()
	req.NoError(err)
	req.Equal(int64(12), info.Size())
	req.NotNil(info.Sys())
	req.False(info.IsDir())
	req.Equal("file1", info.Name())
	req.Equal(os.FileMode(0777), info.Mode())

	// Not using the cache
	info, err = fs.Stat("file1")
	req.NoError(err)
	req.Equal(int64(12), info.Size())

	// Delete the file
	req.NoError(fs.Remove("file1"))

	// Let's see if it still exists
	info, err = fs.Stat("file1")
	req.EqualError(err, os.ErrNotExist.Error())
	req.Nil(info)
}

func TestFileWrite(t *testing.T) {
	fs, _ := setup(t)

	testWriteFile(t, fs, "file-1KB", 1024)

	before := time.Now()

	testWriteFile(t, fs, "file-1MB", 1024*1024)

	// We're only doing the 100MB test if the 1MB took less than 3s
	if time.Since(before) < time.Second*3 {
		testWriteFile(t, fs, "file-100MB", 100*1024*1024)
	}
}

func TestBasic(t *testing.T) {
	fs, req := setup(t)

	f, err := fs.Create("file1")
	req.NoError(err)

	req.Equal("dropbox", fs.Name())
	req.EqualError(fs.Chmod("file1", 0777), ErrNotSupported.Error())
	req.EqualError(fs.Chtimes("file1", time.Now(), time.Now()), ErrNotSupported.Error())
	req.EqualError(fs.Chown("file1", 1, 1), ErrNotSupported.Error())
	req.NoError(f.Sync())
	req.EqualError(f.Truncate(10), ErrNotSupported.Error())
}

func TestDirList(t *testing.T) {
	fs, req := setup(t)
	fs.dirListLimit = 2

	req.NoError(fs.Mkdir("dir1", 0))
	req.NoError(fs.Mkdir("dir2", 0))

	for i := 0; i < 5; i++ {
		f, err := fs.OpenFile(fmt.Sprintf("dir1/file_%d.txt", i), os.O_WRONLY, 0)
		req.NoError(err)

		_, err = f.WriteString(fmt.Sprintf("content %d", i))
		req.NoError(err)

		req.NoError(f.Close())
	}

	dir, err := fs.Open("dir1")
	req.NoError(err)

	defer func() { req.NoError(dir.Close()) }()

	{ // Reading everything
		files, errRead := dir.Readdir(1000)
		req.NoError(errRead)
		req.Len(files, 5)
	}

	dir, err = fs.Open("dir1")
	req.NoError(err)

	{ // Reading everything
		files, errRead := dir.Readdir(2)
		req.NoError(errRead)
		req.Len(files, 2)

		files, errRead = dir.Readdir(10)
		req.NoError(errRead)
		req.Len(files, 3)

		files, errRead = dir.Readdir(10)
		req.NoError(errRead)
		req.Len(files, 0)
	}

	dir, err = fs.Open("dir1")
	req.NoError(err)

	{ // Reading names
		filenames, err := dir.Readdirnames(1000)
		req.NoError(err)
		req.Len(filenames, 5)
	}
}

//nolint: gocyclo
func TestFileSeekBasic(t *testing.T) {
	fs, req := setup(t)

	{ // Writing an initial file
		file, err := fs.OpenFile("file1", os.O_WRONLY, 0777)
		req.NoError(err)

		_, err = file.WriteString("Hello world !")
		req.NoError(err)

		req.NoError(file.Close())
	}

	file, errOpen := fs.Open("file1")
	req.NoError(errOpen)

	buffer := make([]byte, 5)

	{ // Reading the world
		if pos, err := file.Seek(6, io.SeekStart); err != nil || pos != 6 {
			t.Fatal("Could not seek:", err)
		}

		if _, err := file.Read(buffer); err != nil {
			t.Fatal("Could not read buffer:", err)
		}

		if string(buffer) != "world" {
			t.Fatal("Bad fetch:", string(buffer))
		}
	}

	{ // Going 3 bytes backwards
		if pos, err := file.Seek(-3, io.SeekCurrent); err != nil || pos != 8 {
			t.Fatal("Could not seek:", err)
		}

		if _, err := file.Read(buffer); err != io.EOF {
			t.Fatal("Could not read buffer:", err)
		}

		if string(buffer) != "rld !" {
			t.Fatal("Bad fetch:", string(buffer))
		}
	}

	{ // And then going back to the beginning
		if pos, err := file.Seek(1, io.SeekStart); err != nil || pos != 1 {
			t.Fatal("Could not seek:", err)
		}

		if _, err := file.Read(buffer); err != nil {
			t.Fatal("Could not read buffer:", err)
		}

		if string(buffer) != "ello " {
			t.Fatal("Bad fetch:", string(buffer))
		}
	}

	{ // And from the end
		if pos, err := file.Seek(5, io.SeekEnd); err != nil || pos != 8 {
			t.Fatal("Could not seek:", err)
		}

		if _, err := file.Read(buffer); err != io.EOF {
			t.Fatal("Could not read buffer:", err)
		}

		req.Equal("rld !", string(buffer))
	}

	// Let's close it
	req.NoError(file.Close())

	// And do an other seek
	_, err := file.Seek(10, io.SeekStart)
	req.EqualError(err, "File is closed")
}
