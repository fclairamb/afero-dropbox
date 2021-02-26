package dropbox

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"os"
	"testing"

	"github.com/spf13/afero"
)

// nolint: gosec
func testWriteFile(t *testing.T, fs afero.Fs, name string, size int) {
	t.Logf("Working on %s with %d bytes", name, size)

	{ // First we write the file
		t.Log("  Writing file")
		reader1 := NewLimitedReader(rand.New(rand.NewSource(0)), size)

		file, errOpen := fs.OpenFile(name, os.O_WRONLY, 0777)
		if errOpen != nil {
			t.Fatal("Could not open file:", errOpen)
		}

		if _, errWrite := io.Copy(file, reader1); errWrite != nil {
			t.Fatal("Could not write file:", errWrite)
		}

		if errClose := file.Close(); errClose != nil {
			t.Fatal("Couldn't close file", errClose)
		}
	}

	{ // Then we read the file
		t.Log("  Reading file")
		reader2 := NewLimitedReader(rand.New(rand.NewSource(0)), size)

		file, errOpen := fs.OpenFile(name, os.O_RDONLY, 0777)
		if errOpen != nil {
			t.Fatal("Could not open file:", errOpen)
		}

		if ok, err := ReadersEqual(file, reader2); !ok || err != nil {
			t.Fatal("Could not equal reader:", err)
		}

		if errClose := file.Close(); errClose != nil {
			t.Fatal("Couldn't close file", errClose)
		}
	}
}

type LimitedReader struct {
	reader io.Reader
	size   int
	offset int
}

func NewLimitedReader(reader io.Reader, limit int) *LimitedReader {
	return &LimitedReader{
		reader: reader,
		size:   limit,
	}
}

// nolint: wrapcheck
func (r *LimitedReader) Read(buffer []byte) (int, error) {
	maxRead := r.size - r.offset

	if maxRead == 0 {
		return 0, io.EOF
	} else if maxRead < len(buffer) {
		buffer = buffer[0:maxRead]
	}

	read, err := r.reader.Read(buffer)
	if err == nil {
		r.offset += read
	}

	return read, err
}

// Source: rog's code from https://groups.google.com/forum/#!topic/golang-nuts/keG78hYt1I0
// nolint: wrapcheck
func ReadersEqual(r1, r2 io.Reader) (bool, error) {
	const chunkSize = 8 * 1024 // 8 KB
	buf1 := make([]byte, chunkSize)
	buf2 := make([]byte, chunkSize)

	for {
		n1, err1 := io.ReadFull(r1, buf1)
		n2, err2 := io.ReadFull(r2, buf2)

		if err1 != nil && !errors.Is(err1, io.EOF) && !errors.Is(err1, io.ErrUnexpectedEOF) {
			return false, err1
		}

		if err2 != nil && !errors.Is(err2, io.EOF) && !errors.Is(err2, io.ErrUnexpectedEOF) {
			return false, err2
		}

		if (err1 != nil) != (err2 != nil) || !bytes.Equal(buf1[0:n1], buf2[0:n2]) {
			return false, nil
		}

		if err1 != nil {
			return true, nil
		}
	}
}
