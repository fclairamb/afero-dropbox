package dropbox // nolint: golint

import "errors"

// ErrNotSupported is returned when this operations is not supported by S3.
var ErrNotSupported = errors.New("dropbox doesn't support this operation")

// ErrAlreadyOpened is returned when the file is already opened.
var ErrAlreadyOpened = errors.New("already opened")

// ErrInvalidSeek is returned when the seek operation is not doable.
var ErrInvalidSeek = errors.New("invalid seek offset")
