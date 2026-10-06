package block

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const (
	headerSize = 8
	seqSize    = 12
)

var (
	ErrCorrupt = errors.New("lz77: corrupt block")
)

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
