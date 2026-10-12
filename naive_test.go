package block

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// this is avoid the error cannot find on hash chain
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

func FuzzGreddyHashChain(f *testing.F) {
	f.Add([]byte(""), []byte("abcabcabcX"))
	f.Add([]byte("xyz"), []byte("xyzxyzxyz"))

	f.Fuzz(func(t *testing.T, hist, src []byte) {
		if len(src) > 4<<10 {
			src = src[:4<<10]
		}

		greddyBlk := compressNaive(hist, src, 3)
		hashChainBlk := compressHC(hist, src, 3, math.MaxInt)

		assert.Equal(t, greddyBlk.Literals, hashChainBlk.Literals)
		assert.Equal(t, greddyBlk.Seqs, hashChainBlk.Seqs)
		assert.Equal(t, greddyBlk.TailLen, hashChainBlk.TailLen)
	})
}

func TestLongestMatch(t *testing.T) {
	buf := []byte{0, 1, 1, 1, 2, 2, 3, 4, 5, 3, 3, 3, 3, 3, 1}
	l, off := longestMatch(10, buf)
	if l != 4 || off != 1 {
		t.Errorf("longestMatch(%d, %q) = %d, %d, want %d, %d", 0, buf, l, off, 4, 3)
	}

	buf = []byte{0, 1, 2, 3, 1, 2, 3}
	l, off = longestMatch(4, buf)
	if l != 3 || off != 3 {
		t.Errorf("longestMatch(%d, %q) = %d, %d, want %d, %d", 0, buf, l, off, 3, 0)
	}
}
