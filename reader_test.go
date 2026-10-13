package block

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_Header(t *testing.T) {
	t.Run("the header is correct", func(t *testing.T) {
		var buf bytes.Buffer

		writer := NewWriter(&buf)
		assert.Equal(t, writer.hasHeader, false)
		err := writer.writeHeader()
		assert.Nil(t, err)
		assert.Equal(t, writer.hasHeader, true)
		err = verify(buf.Bytes()[:headerLen])
		assert.Nil(t, err)
	})

	t.Run("the magic error", func(t *testing.T) {
		var buf bytes.Buffer

		writer := NewWriter(&buf)
		assert.Equal(t, writer.hasHeader, false)
		err := writer.writeHeader()
		assert.Nil(t, err)
		header := make([]byte, headerLen)
		copy(header, buf.Bytes()[:headerLen])
		header[0] = 'a'
		err = verify(header)
		assert.Equal(t, err, ErrMagic)
	})

	t.Run("the version error", func(t *testing.T) {
		var buf bytes.Buffer
		writer := NewWriter(&buf)
		assert.Equal(t, writer.hasHeader, false)
		err := writer.writeHeader()
		assert.Nil(t, err)
		header := make([]byte, headerLen)
		copy(header, buf.Bytes()[:headerLen])
		header[len(MAGIC)] = VERSION + 1
		err = verify(header)
		assert.Equal(t, err, ErrVersion)
	})
}

func FuzzHeader(f *testing.F) {
	compress := func(src []byte) []byte {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		if _, err := w.Write(src); err != nil {
			f.Fatal(err)
		}
		if err := w.Close(); err != nil {
			f.Fatal(err)
		}
		return buf.Bytes()
	}

	f.Add(compress(nil))
	f.Add(compress([]byte("hello hello")))
	f.Add(compress(bytes.Repeat([]byte("abcab"), blockSize/5+7)))

	header := []byte(MAGIC + string(rune(VERSION)))
	f.Add(header[:3])
	f.Add(slices.Concat(header, []byte{1, 0}))
	f.Add(slices.Concat(header, []byte{0xff, 0xff, 0xff, 0xff}))
	f.Add(slices.Concat(header, []byte{10, 0, 0, 0}, []byte{1}))

	f.Fuzz(func(t *testing.T, a []byte) {
		_, err := io.ReadAll(NewReader(bytes.NewReader(a)))
		if err != nil && !errors.Is(err, ErrCorrupt) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func FuzzReaderCRC(f *testing.F) {
	f.Add([]byte("hello"), uint16(0), byte(1))
	f.Add([]byte("abcdabcdabcd"), uint16(20), byte(0x80))

	f.Fuzz(func(t *testing.T, data []byte, pos uint16, flip byte) {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		_, err := w.Write(data)
		require.NoError(t, err)
		require.NoError(t, w.Close())

		b := buf.Bytes()
		if len(b) == 0 || flip == 0 {
			return
		}

		b[int(pos)%len(b)] ^= flip
		got, err := io.ReadAll(NewReader(bytes.NewReader(b)))
		assert.Error(t, err)
		assert.NotEqual(t, got, data, "the cheksum should not equal")
	})
}
