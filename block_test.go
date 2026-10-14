package block

import (
	"bytes"
	"io"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func FuzzRoundTripHC(f *testing.F) {
	f.Add([]byte(""), []byte("abcabcabcX"))
	f.Add([]byte("xyz"), []byte("xyzxyzxyz"))

	f.Fuzz(func(t *testing.T, hist, src []byte) {
		if len(src) > 4<<10 {
			src = src[:4<<10]
		}

		blk := compressHC(hist, src, 3)
		got, err := blk.Decode(hist)
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if !bytes.Equal(got, src) {
			t.Fatalf("decode error: got %q, want %q", got, src)
		}
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
	c                              symCounts
	ll                             [32]uint32 // litLen codes
	extraBits                      int
}

func entropyBits(counts []uint32) (b float64) {
	var total float64
	for _, c := range counts {
		total += float64(c)
	}
	for _, c := range counts {
		if c > 0 {
			b -= float64(c) * math.Log2(float64(c)/total)
		}
	}
	return
}

func (st *stats) ratio() float64 {
	bits := entropyBits(st.c.lit[:]) + entropyBits(st.c.ml[:]) +
		entropyBits(st.c.of[:]) + entropyBits(st.ll[:]) + float64(st.extraBits)
	return bits / 8 / float64(st.in)
}

func measure(t *testing.T, data []byte, minMatch int, compress func(hist, src []byte, minMatch int) Block) (st stats) {
	const blockSize = 128 << 10
	var hist []byte
	for off := 0; off < len(data); off += blockSize {
		src := data[off:min(off+blockSize, len(data))]
		blk := compress(hist, src, minMatch)

		got, err := blk.Decode(hist)
		if err != nil || !bytes.Equal(got, src) {
			t.Fatalf("block @%d round-trip failed：%v", off, err)
		}

		st.literals += len(blk.Literals)
		st.seqs += len(blk.Seqs)
		for _, b := range blk.Literals {
			st.c.lit[b]++
		}
		for _, s := range blk.Seqs {
			st.matchBytes += int(s.MatchLen)
			mc, mx := mlCode(int(s.MatchLen), minMatch)
			oc, ox := offCode(int(s.Offset))
			lc, lx := mlCode(int(s.LitLen), 0)
			st.c.ml[mc]++
			st.c.of[oc]++
			st.ll[lc]++
			st.extraBits += mx + ox + lx
		}
		end := off + len(src)
		hist = data[max(0, end-windowSize):end]
	}
	st.in = len(data)
	return
}

func runMeasure(t *testing.T, compress func(hist, src []byte, minMatch int) Block) {
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
		st := measure(t, data, mm, compress)
		el := time.Since(start)

		t.Logf("minMatch=%d ratio=%.4f seqs=%d avgMatch=%.1f literals=%d time=%v (%.3f MB/s)",
			mm,
			st.ratio(),
			st.seqs,
			float64(st.matchBytes)/float64(max(st.seqs, 1)),
			st.literals,
			el.Round(time.Millisecond),
			float64(st.in)/1e6/el.Seconds(),
		)
	}
}

func TestMeasure_DP(t *testing.T) { runMeasure(t, compressDP) }

func TestMeasure_HC(t *testing.T) {
	runMeasure(t, func(hist, src []byte, mm int) Block { return compressHC(hist, src, mm) })
}

func TestStreamRoundTrip(t *testing.T) {
	for _, size := range []int{0, 1, 100, blockSize - 1, blockSize, 3*blockSize + 7} {
		src := make([]byte, size)
		for i := range src {
			src[i] = "abcab"[i%5] ^ byte(i/1000)
		}

		var buf bytes.Buffer
		w := NewWriter(&buf)
		for p := src; len(p) > 0; {
			n := min(len(p), 777)
			_, err := w.Write(p[:n])
			require.NoError(t, err)
			p = p[n:]
		}
		require.NoError(t, w.Close())

		got, err := io.ReadAll(NewReader(&buf))
		require.NoError(t, err)
		assert.True(t, bytes.Equal(src, got), "size=%d", size)
	}
}

func FuzzRoundTrip(f *testing.F) {
	noise := make([]byte, 4096)
	x := uint32(1)
	for i := range noise {
		x = x*1664525 + 1013904223
		noise[i] = byte(x >> 24)
	}

	periodic := func(n int) []byte {
		return bytes.Repeat([]byte("abcab"), n/5+1)[:n]
	}

	f.Add([]byte{}, uint16(1))
	f.Add([]byte("a"), uint16(0))
	f.Add([]byte("hello hello hello"), uint16(1))
	f.Add([]byte("hello hello hello"), uint16(1000))
	f.Add(noise, uint16(777))

	f.Add(periodic(blockSize-1), uint16(777))
	f.Add(periodic(blockSize), uint16(65535))
	f.Add(periodic(blockSize+1), uint16(65535))
	f.Add(periodic(2*blockSize+7), uint16(4096))

	f.Fuzz(func(t *testing.T, data []byte, chunk uint16) {
		size := max(int(chunk), 1)
		var buf bytes.Buffer
		w := NewWriter(&buf)

		for p := data; len(p) > 0; {
			n := min(len(p), size)
			_, err := w.Write(p[:n])
			assert.Nil(t, err)
			p = p[n:]
		}

		err := w.Close()
		assert.Nil(t, err)
		got, err := io.ReadAll(NewReader(&buf))
		assert.Nil(t, err)
		assert.Equal(t, got, data)
	})
}

func TestStreamDetectsChecksumMismatch(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	_, err := w.Write([]byte("hello hello hello"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	b := buf.Bytes()
	b[len(b)-1] ^= 0xff

	_, err = io.ReadAll(NewReader(bytes.NewReader(b)))
	assert.ErrorIs(t, err, ErrChecksum)
}
