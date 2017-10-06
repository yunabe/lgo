package main

import (
	"errors"
	"io"
	"unicode/utf8"
)

var errBufTooSmall = errors.New("buf is too small")

type utf8AwareReader struct {
	reader     io.Reader
	residual   []byte
	pendingErr error
}

func newUTF8AwareReader(r io.Reader) *utf8AwareReader {
	return &utf8AwareReader{
		reader:   r,
		residual: make([]byte, 0, utf8.UTFMax-1),
	}
}

func (r *utf8AwareReader) Read(p []byte) (int, error) {
	if r.pendingErr != nil {
		err := r.pendingErr
		r.pendingErr = nil
		return 0, err
	}
	if len(p) < utf8.UTFMax*2 {
		return 0, errBufTooSmall
	}
	if len(p) <= len(r.residual) {
		panic("r.residual must be smaller than utf8.UTFMax")
	}
	copy(p, r.residual)
	n, err := r.reader.Read(p[len(r.residual):])

	if n == 0 && err != nil && len(r.residual) > 0 {
		r.pendingErr = err
		copy(p, r.residual)
		n = len(r.residual)
		r.residual = r.residual[:0]
		return n, nil
	}
	n += len(r.residual)
	if err != nil {
		// e.g. io.EOF
		r.residual = r.residual[:0]
		return n, err
	}
	for i := 0; i < utf8.UTFMax && i < n; i++ {
		ru, _ := utf8.DecodeLastRune(p[:n-i])
		if ru != utf8.RuneError {
			r.residual = r.residual[:i]
			copy(r.residual, p[n-i:])
			return n - i, nil
		}
	}
	// The last utf8.UTFMax bytes are invalid as UTF8. It means the data is not valid UTF8 string.
	// Return everthing.
	return n, nil
}
