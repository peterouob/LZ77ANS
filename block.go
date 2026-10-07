package block

import (
	"errors"
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

	buf := slices.Concat(hits)
	lits := b.Literals

	for _, seq := range b.Seqs {
		if int(seq.LitLen) > len(lits) {
			return nil, ErrLiterLensOverflow
		}

		buf = slices.Concat(buf, lits[:seq.LitLen])
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
			b.Literals = slices.Concat(b.Literals, buf[litStart:cur])
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

	return b
}

func compressNaive(hits, src []byte, minMatch int) Block {
	b := Block{}

	totalSize := len(hits) + len(src)
	buf := make([]byte, totalSize)

	copy(buf, hits)
	copy(buf[len(hits):], src)

	cur := len(hits)
	litStart := cur

	for cur < totalSize {
		bestLen, bestOff := longestMatch(cur, buf)

		if bestLen >= minMatch {
			b.Literals = slices.Concat(b.Literals, buf[litStart:cur])
			b.Seqs = append(b.Seqs, Sequence{
				LitLen:   uint32(cur - litStart),
				MatchLen: uint32(bestLen),
				Offset:   uint32(bestOff),
			})

			b.Seqs = slices.Concat(b.Seqs)

			cur += bestLen
			litStart = cur

			continue
		}

		cur++
	}

	tail := buf[litStart:]
	b.Literals = slices.Concat(b.Literals, tail)
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
