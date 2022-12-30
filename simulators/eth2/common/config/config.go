package config

import (
	"bytes"
	"io"
	"io/ioutil"
)

func BytesSource(data []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	}
}
