# Dropbox Backend for Afero

[![Go version](https://img.shields.io/github/go-mod/go-version/fclairamb/afero-dropbox)](https://golang.org/doc/devel/release.html)
![Build](https://github.com/fclairamb/afero-dropbox/workflows/Build/badge.svg)
[![codecov](https://codecov.io/gh/fclairamb/afero-dropbox/branch/main/graph/badge.svg?token=BdvUdgu9E3)](https://codecov.io/gh/fclairamb/afero-dropbox)
[![Go Report Card](https://goreportcard.com/badge/fclairamb/afero-dropbox)](https://goreportcard.com/report/fclairamb/afero-dropbox)
[![GoDoc](https://godoc.org/github.com/fclairamb/afero-dropbox?status.svg)](https://godoc.org/github.com/fclairamb/afero-dropbox)


## About
It provides an [afero filesystem](https://github.com/spf13/afero/) implementation of a [Dropbox](https://www.dropbox.com/) backend.

This was created to provide a backend to the [ftpserver](https://github.com/fclairamb/ftpserver) but can definitely be used in any other code.

I'm very opened to any improvement through issues or pull-request that might lead to a better implementation or even
better testing.

It is implemented on top of the [Dropbox Go SDK](https://github.com/dropbox/dropbox-sdk-go-unofficial).

## Key points
- Download & upload file streaming
- _Some_ coverage (all APIs are tested, but not all errors are reproduced)
- Very carefully linted

## Known limitations
- File appending / seeking for write is not supported because dropbox doesn't support it
- Chmod / Chtimes are not supported because dropbox doesn't support it

## How to use
Note: Errors handling is skipped for brevity, but you definitely have to handle it.

```golang
package main

import(
	"os"
	
	dropbox "github.com/fclairamb/afero-dropbox"
)

func main() {
  // You access an FS with it's token
  fs := dropbox.NewFs(os.Getenv("DROPBOX_TOKEN"))
  
  // And do your thing
  file, _ := fs.OpenFile("file.txt", os.O_WRONLY, 0777)
  file.WriteString("Hello world !")
  file.Close()
}
```
