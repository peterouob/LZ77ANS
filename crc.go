package block

import (
	"encoding/binary"
	"hash/crc32"
	"math/bits"
)

var table = crc32.MakeTable(crc32.Castagnoli)

const crcMagic = 0xa282ead8

type CRC uint32

func NewCRC(data []byte) CRC {
	return CRC(0).update(data)
}

func (c CRC) update(data []byte) CRC {
	return CRC(crc32.Update(uint32(c), table, data))
}

func (c CRC) Value() uint32 {
	return bits.RotateLeft32(uint32(c), -15) + crcMagic
}

func (c CRC) ByteValue() []byte {
	return binary.LittleEndian.AppendUint32(nil, c.Value())
}
