package block

import (
	"bytes"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func FuzzRoundTripNavie(f *testing.F) {
	f.Add([]byte(""), []byte("abcabcabcX"))
	f.Add([]byte("xyz"), []byte("xyzxyzxyz"))

	f.Fuzz(func(t *testing.T, hist, src []byte) {
		if len(src) > 4<<10 {
			src = src[:4<<10]
		}

		blk := compressNaive(hist, src, 3)
		got, err := blk.Decode(hist)
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if !bytes.Equal(got, src) {
			t.Fatalf("decode error: got %q, want %q", got, src)
		}
	})
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

func TestCompressHC(t *testing.T) {
	src := []byte("xyzxyzxyz")
	hits := []byte("xy")
	b := compressHC(hits, src, 3)
	got, err := b.Decode(hits)
	assert.Nil(t, err)
	assert.Equal(t, got, src)
}

func TestCompressBlock(t *testing.T) {
	src := []byte("xyzxyzxyz")
	hits := []byte("xy")
	b := compressNaive(hits, src, 3)
	got, err := b.Decode(hits)
	assert.Nil(t, err)
	assert.Equal(t, got, src)
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

func TestHashChainWindowEdge(t *testing.T) {
	hist := make([]byte, windowSize+16)
	x := uint32(1)
	for i := range hist {
		x = x*1664525 + 1013904223
		hist[i] = byte(x >> 24)
	}

	marker := []byte("MARKER!!")
	edge := len(hist) - windowSize
	copy(hist[edge:], marker)
	copy(hist[edge-10:edge-2], marker)

	src := bytes.Clone(marker)
	want := compressNaive(hist, src, 3)
	got := compressHC(hist, src, 3, math.MaxInt)

	require.Len(t, got.Seqs, 1)
	assert.Equal(t, uint32(windowSize), got.Seqs[0].Offset)
	assert.Equal(t, want.Seqs, got.Seqs)
}

type stats struct {
	in, literals, seqs, matchBytes int
}

func measure(t *testing.T, data []byte, minMatch int) (st stats) {
	const blockSize = 128 << 10
	var hist []byte
	for off := 0; off < len(data); off += blockSize {
		src := data[off:min(off+blockSize, len(data))]
		blk := compressNaive(hist, src, minMatch)

		got, err := blk.Decode(hist)
		if err != nil || !bytes.Equal(got, src) {
			t.Fatalf("block @%d round-trip failed：%v", off, err)
		}

		st.literals += len(blk.Literals)
		st.seqs += len(blk.Seqs)
		for _, s := range blk.Seqs {
			st.matchBytes += int(s.MatchLen)
		}
		end := off + len(src)
		hist = data[max(0, end-windowSize):end]
	}
	st.in = len(data)
	return
}

func measureHC(t *testing.T, data []byte, minMatch int) (st stats) {
	const blockSize = 128 << 10
	var hist []byte
	for off := 0; off < len(data); off += blockSize {
		src := data[off:min(off+blockSize, len(data))]
		blk := compressHC(hist, src, minMatch)

		got, err := blk.Decode(hist)
		if err != nil || !bytes.Equal(got, src) {
			t.Fatalf("block @%d round-trip failed：%v", off, err)
		}

		st.literals += len(blk.Literals)
		st.seqs += len(blk.Seqs)
		for _, s := range blk.Seqs {
			st.matchBytes += int(s.MatchLen)
		}
		end := off + len(src)
		hist = data[max(0, end-windowSize):end]
	}
	st.in = len(data)
	return
}

func TestMeasure_Naive(t *testing.T) {
	path := os.Getenv("LZ77_CORPUS")
	if path == "" {
		t.Skip("need to set LZ77 destination")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, mm := range []int{3, 4, 5} {
		start := time.Now()
		st := measure(t, data, mm)
		el := time.Since(start)

		bits := st.literals*9 + st.seqs*24 // cost for spec
		t.Logf("minMatch=%d ratio=%.4f seqs=%d avgMatch=%.1f literals=%d time=%v (%.3f MB/s)",
			mm,
			float64(bits)/8/float64(st.in),
			st.seqs,
			float64(st.matchBytes)/float64(max(st.seqs, 1)),
			st.literals,
			el.Round(time.Millisecond),
			float64(st.in)/1e6/el.Seconds(),
		)
	}
}

func TestMeasure_HC(t *testing.T) {
	path := os.Getenv("LZ77_CORPUS")
	if path == "" {
		t.Skip("need to set LZ77 destination")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, mm := range []int{3, 4, 5} {
		start := time.Now()
		st := measureHC(t, data, mm)
		el := time.Since(start)

		bits := st.literals*9 + st.seqs*24 // cost for spec
		t.Logf("minMatch=%d ratio=%.4f seqs=%d avgMatch=%.1f literals=%d time=%v (%.3f MB/s)",
			mm,
			float64(bits)/8/float64(st.in),
			st.seqs,
			float64(st.matchBytes)/float64(max(st.seqs, 1)),
			st.literals,
			el.Round(time.Millisecond),
			float64(st.in)/1e6/el.Seconds(),
		)
	}
}
