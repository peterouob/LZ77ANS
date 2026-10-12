package block

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
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
	f.Add()
}
