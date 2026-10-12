package block

import (
	"encoding/binary"
	"hash/crc32"
	"math/bits"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCRC(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		buf := []byte("hello")
		want := bits.RotateLeft32(crc32.Checksum(buf, table), -15) + crcMagic
		got := NewCRC(buf).Value()

		assert.Equal(t, want, got, "CRC checksum mismatch")
	})

	t.Run("byte value", func(t *testing.T) {
		buf := []byte("hello")
		want := binary.LittleEndian.AppendUint32(nil, bits.RotateLeft32(crc32.Checksum(buf, table), -15)+crcMagic)
		got := NewCRC(buf).ByteValue()

		assert.Equal(t, want, got, "CRC checksum mismatch")
	})
}
