package block

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
)

type Reader struct {
	r      io.Reader
	header bool
	hist   []byte
	out    []byte
	body   []byte
	err    error
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

func (r *Reader) Read(p []byte) (int, error) {
	for len(r.out) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		r.err = r.readBlock()
	}

	n := copy(p, r.out)
	r.out = r.out[n:]
	return n, nil
}

func (r *Reader) readHeader() error {
	if r.header {
		return nil
	}

	header := make([]byte, headerLen)
	if n, err := io.ReadFull(r.r, header); err != nil {
		switch {
		case errors.Is(err, io.EOF) || n == 0:
			return io.EOF
		case errors.Is(err, io.ErrUnexpectedEOF):
			return ErrCorrupt
		default:
			return err
		}
	}

	if err := verify(header); err != nil {
		return err
	}

	r.header = true

	return nil
}

func (r *Reader) readBlock() error {

	if err := r.readHeader(); err != nil {
		return err
	}

	lenBuf := make([]byte, 4)

	if _, err := io.ReadFull(r.r, lenBuf); err != nil {
		switch {
		case errors.Is(err, io.EOF):
			return io.EOF
		case errors.Is(err, io.ErrUnexpectedEOF):
			return ErrCorrupt
		default:
			return err
		}
	}

	bodyLen := binary.LittleEndian.Uint32(lenBuf)

	if bodyLen > maxBodyLen {
		return ErrCorrupt
	}

	r.body = slices.Grow(r.body[:0], int(bodyLen))[:bodyLen]
	if _, err := io.ReadFull(r.r, r.body); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return ErrCorrupt
		}

		return err
	}

	block, err := parseCompressedBody(r.body)
	if err != nil {
		return err
	}

	origin, err := block.Decode(r.hist)
	if err != nil {
		return err
	}

	r.out = origin

	r.hist = slices.Concat(r.hist, origin)

	if len(r.hist) > windowSize {
		r.hist = slices.Concat(r.hist[:0], r.hist[len(r.hist)-windowSize:])
	}

	return nil
}

var (
	ErrMagic   = fmt.Errorf("%w: bad magic in header", ErrCorrupt)
	ErrVersion = fmt.Errorf("%w: bad version in header", ErrCorrupt)
)

func verify(header []byte) error {
	magic := header[:len(MAGIC)]
	version := header[len(MAGIC)]

	if string(magic) != MAGIC {
		return ErrMagic
	}

	if version != VERSION {
		return ErrVersion
	}

	return nil
}
