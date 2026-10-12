package block

import (
	"fmt"
	"slices"
)

const (
	minMatchLen  = 3
	maxMatchLen  = 258
	maxHashChain = 129
	windowSize   = 32 * 1024
	blockSize    = 128 * 1024
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

	checkSum CRC
}

var (
	ErrLiterLensOverflow = fmt.Errorf("%w: literal lens overflow", ErrCorrupt)
	ErrMatchLensOverflow = fmt.Errorf("%w: match lens overflow", ErrCorrupt)
	ErrOffsetOverflow    = fmt.Errorf("%w: offset overflow", ErrCorrupt)
	ErrTailLensNotEqual  = fmt.Errorf("%w: tail lens not equal", ErrCorrupt)
	ErrBlockTooLarge     = fmt.Errorf("%w: block too lerage", ErrCorrupt)
)

func (b *Block) Decode(hits []byte) ([]byte, error) {
	if len(hits) > windowSize {
		hits = hits[len(hits)-windowSize:]
	}

	var outSize, litSum uint64
	for _, seq := range b.Seqs {
		if seq.MatchLen > maxMatchLen || seq.MatchLen < minMatchLen {
			return nil, ErrOffsetOverflow
		}

		outSize += uint64(seq.LitLen) + uint64(seq.MatchLen)
		litSum += uint64(seq.LitLen)

		if outSize > blockSize {
			return nil, ErrBlockTooLarge
		}
	}

	outSize += uint64(b.TailLen)
	if outSize > blockSize {
		return nil, ErrBlockTooLarge
	}

	if litSum+uint64(b.TailLen) != uint64(len(b.Literals)) {
		return nil, ErrTailLensNotEqual
	}

	buf := make([]byte, 0, len(hits)+int(outSize))
	buf = append(buf, hits...)
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

	buf = slices.Concat(buf, lits)
	return buf[len(hits):], nil
}

func compressHC(hits, src []byte, minMatch int, maxChain ...int) Block {
	b := Block{}
	totalSize := len(hits) + len(src)
	buf := make([]byte, totalSize)

	if maxChain == nil {
		maxChain = []int{maxHashChain}
	}

	chain := HashChain{MaxChain: maxChain[0]}

	copy(buf, hits)
	copy(buf[len(hits):], src)

	for i := max(0, len(hits)-windowSize); i < len(hits); i++ {
		chain.Insert(buf, i)
	}

	cur := len(hits)
	litStart := cur

	for cur < totalSize {
		bestLen, bestOff := chain.longestMatch(buf, cur)
		chain.Insert(buf, cur)

		if bestLen >= minMatch {
			b.Literals = append(b.Literals, buf[litStart:cur]...)
			b.Seqs = append(b.Seqs, Sequence{
				LitLen:   uint32(cur - litStart),
				MatchLen: uint32(bestLen),
				Offset:   uint32(bestOff),
			})

			for c := cur + 1; c < cur+bestLen; c++ {
				chain.Insert(buf, c)
			}

			cur += bestLen
			litStart = cur

			continue
		}

		cur++
	}

	tail := buf[litStart:]
	b.Literals = slices.Concat(b.Literals, tail)
	b.TailLen = uint32(len(tail))

	b.checkSum = CRC(NewCRC(src).Value())
	return b
}
