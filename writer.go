package block

import (
	"encoding/binary"
	"errors"
	"io"
	"slices"
)

const (
	MAGIC      = "peter"
	VERSION    = 1
	headerLen  = len(MAGIC) + 1
	maxBodyLen = 8 + 4*blockSize
)

var ErrClosed = errors.New("lw77: writer closed")

type Level int

const (
	levelHC Level = iota
	levelDP
)

type window struct {
	buf  []byte
	hist []byte
}

func (w *window) full() bool {
	return len(w.buf) >= blockSize
}

func (w *window) fill(p []byte) int {
	n := min(len(p), blockSize-len(w.buf))
	w.buf = append(w.buf, p[:n]...)
	return n
}

func (w *window) slide() {
	w.hist = slices.Concat(w.hist, w.buf)
	if len(w.hist) > windowSize {
		w.hist = slices.Concat(w.hist[:0], w.hist[len(w.hist)-windowSize:])
	}

	w.buf = w.buf[:0]
}

type Writer struct {
	w         io.Writer
	win       window
	frame     []byte
	err       error
	hasHeader bool
	level     Level
}

func NewWriter(w io.Writer, level ...Level) *Writer {
	l := levelHC
	if len(level) != 0 {
		l = levelDP
	}

	return &Writer{w: w, level: l}
}

func (w *Writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}

	total := len(p)
	for len(p) > 0 {
		n := w.win.fill(p)
		p = p[n:]

		if !w.win.full() {
			continue
		}

		if w.err = w.writeBlock(); w.err != nil {
			return total - len(p), w.err
		}
	}

	return total, nil
}

func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	if len(w.win.buf) == 0 {
		return nil
	}
	w.err = w.writeBlock()
	return w.err
}

func (w *Writer) Close() error {
	if errors.Is(w.err, ErrClosed) {
		return nil
	}

	if err := w.writeHeader(); err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return err
	}
	w.err = ErrClosed
	return nil
}

func (w *Writer) writeHeader() error {
	if w.hasHeader {
		return nil
	}

	header := slices.Concat([]byte(MAGIC), []byte{VERSION})
	if _, err := w.w.Write(header); err != nil {
		return err
	}

	w.hasHeader = true
	return nil
}

func (w *Writer) writeBlock() error {
	if err := w.writeHeader(); err != nil {
		return err
	}

	var block Block
	switch w.level {
	case 0:
		block = compressHC(w.win.hist, w.win.buf, 3)
	case 1:
		block = compressDP(w.win.hist, w.win.buf, 3)
	}

	w.frame = slices.Grow(w.frame[:0], 4+maxBodyLen)
	w.frame = w.frame[:4]
	body, err := appendCompressedBody(w.frame, &block)
	if err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(body[:4], uint32(len(body))-4)
	if _, err := w.w.Write(body); err != nil {
		return err
	}

	w.win.slide()
	return nil
}
