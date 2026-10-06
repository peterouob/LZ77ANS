package block

import (
	"bytes"
	"encoding/binary"
	"errors"
)

type blockType uint8

const (
	typeRaw blockType = iota
	typeRLE
	typeCompressed
)

const (
	headerSize = 8
	seqSize    = 12
)

var (
	ErrBadMagic   = errors.New("lz77: bad magic")
	ErrBadVersion = errors.New("lz77: unsupported version")
	ErrClosed     = errors.New("lz77: write after close")
	ErrCorrupt    = errors.New("lz77: corrupt block")
)

func putBlockHeader(dst []byte, last bool, t blockType, size int) {
	v := uint32(size)<<3 | uint32(t)<<1
	if last {
		v |= 1
	}
	dst[0], dst[1], dst[2] = byte(v), byte(v>>8), byte(v>>16)
}

func parseBlockHeader(src []byte) (last bool, t blockType, size int) {
	v := uint32(src[0]) | uint32(src[1])<<8 | uint32(src[2])<<16
	return v&1 == 1, blockType(v >> 1 & 3), int(v >> 3)
}

func appendCompressedBody(dst []byte, b *Block) ([]byte, error) {
	var sumLitLen uint64
	for _, seq := range b.Seqs {
		sumLitLen += uint64(seq.LitLen)
	}

	if uint64(len(b.Literals)) != sumLitLen+uint64(b.TailLen) {
		return nil, ErrCorrupt
	}

	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(b.Seqs)))
	dst = binary.LittleEndian.AppendUint32(dst, b.TailLen)

	for _, seq := range b.Seqs {
		dst = binary.LittleEndian.AppendUint32(dst, seq.LitLen)
		dst = binary.LittleEndian.AppendUint32(dst, seq.MatchLen)
		dst = binary.LittleEndian.AppendUint32(dst, seq.Offset)
	}

	dst = append(dst, b.Literals...)

	return dst, nil
}

func parseCompressedBody(body []byte) (*Block, error) {
	if len(body) < 8 {
		return nil, ErrCorrupt
	}

	seqCount := uint64(binary.LittleEndian.Uint32(body))
	tailLen := binary.LittleEndian.Uint32(body[4:])

	lenInfo := headerSize + seqSize*seqCount

	if lenInfo > uint64(len(body)) {
		return nil, ErrCorrupt
	}

	seqs := make([]Sequence, seqCount)
	var sumLitLen uint64

	p := body[headerSize:lenInfo]
	for i := range seqCount {
		s := p[i*seqSize : (i+1)*seqSize]
		seqs[i] = Sequence{
			LitLen:   binary.LittleEndian.Uint32(s[:4]),
			MatchLen: binary.LittleEndian.Uint32(s[4:8]),
			Offset:   binary.LittleEndian.Uint32(s[8:12]),
		}

		sumLitLen += uint64(seqs[i].LitLen)
	}

	lits := body[lenInfo:]

	if uint64(len(lits)) != sumLitLen+uint64(tailLen) {
		return nil, ErrCorrupt
	}

	b := &Block{
		Literals: bytes.Clone(lits),
		Seqs:     seqs,
		TailLen:  tailLen,
	}

	return b, nil
}
