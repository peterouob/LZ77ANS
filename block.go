package block

import (
	"errors"
)

const (
	minMatchLen = 3
	maxMatchLen = 258
	windowSize  = 32 * 1024
	_blockSize  = 128 * 1024
)

type Sequence struct {
	LitLen   uint32
	MatchLen uint32
	Offset   uint32
}

type Block struct {
	Literals []byte
	Seqs     []Sequence
	TailLen  uint32
}

var (
	ErrLiterLensOverflow = errors.New("literal lens overflow")
	ErrMatchLensOverflow = errors.New("match lens overflow")
	ErrOffsetOverflow    = errors.New("offset overflow")
	ErrTailLensNotEqual  = errors.New("tail lens not equal")
)

func (b *Block) Decode(hits []byte) ([]byte, error) {
	if len(hits) > windowSize {
		hits = hits[len(hits)-windowSize:]
	}

	buf := append([]byte(nil), hits...)
	lits := b.Literals

	for _, seq := range b.Seqs {
		if int(seq.LitLen) > len(lits) {
			return nil, ErrLiterLensOverflow
		}

		buf = append(buf, lits[:seq.LitLen]...)
		lits = lits[seq.LitLen:]

		if seq.MatchLen > maxMatchLen || seq.MatchLen < minMatchLen {
			return nil, ErrMatchLensOverflow
		}

		if seq.Offset < 1 || int(seq.Offset) > min(len(buf), windowSize) {
			return nil, ErrOffsetOverflow
		}

		src := len(buf) - int(seq.Offset)
		for i := range seq.MatchLen {
			buf = append(buf, buf[src+int(i)])
		}
	}

	if len(lits) != int(b.TailLen) {
		return nil, ErrTailLensNotEqual
	}

	buf = append(buf, lits...)
	return buf[len(hits):], nil
}

func compressNaive(hits, src []byte) Block {
	b := Block{}

	totalSize := len(hits) + len(src)
	buf := make([]byte, totalSize)

	copy(buf, hits)
	copy(buf[len(hits):], src)

	cur := len(hits)
	litStart := cur

	for cur < totalSize {
		bestLen, bestOff := longestMatch(cur, buf)

		if bestLen >= minMatchLen {
			b.Literals = append(b.Literals, buf[litStart:cur]...)
			b.Seqs = append(b.Seqs, Sequence{
				LitLen:   uint32(cur - litStart),
				MatchLen: uint32(bestLen),
				Offset:   uint32(bestOff),
			})

			cur += bestLen
			litStart = cur

			continue
		}

		cur++
	}

	tail := buf[litStart:]
	b.Literals = append(b.Literals, tail...)
	b.TailLen = uint32(len(tail))

	return b
}

func longestMatch(cur int, buf []byte) (bestLen, bestOff int) {
	maxLen := max(0, cur-windowSize)

	for prev := cur - 1; prev >= maxLen; prev-- {
		n := 0
		for (cur+n) < len(buf) && n < maxMatchLen && buf[prev+n] == buf[cur+n] {
			n++
		}

		if n > bestLen {
			bestLen, bestOff = n, cur-prev
		}

		if bestLen > maxMatchLen {
			break
		}
	}
	return
}
