package block

import (
	"math"
	"math/bits"
	"slices"
)

type match struct {
	len uint16
	off uint32
}

type Price struct {
	minMatch int
	lit      [256]uint32
	of       [numOffCodes]uint32
	ml       [numMLCodes]uint32
	mlPrice  [maxMatchLen + 1]uint32
}

type symCounts struct {
	lit [256]uint32
	ml  [numMLCodes]uint32
	of  [numOffCodes]uint32
}

const alpha = 0.5

func priceSmooth(counts, dst []uint32) {
	total := alpha * float64(len(counts))
	for _, c := range counts {
		total += float64(c)
	}

	for i, c := range counts {
		p := (float64(c) + alpha) / total
		dst[i] = uint32(-math.Log2(p) * 256)
	}
}

func (p *Price) update(c *symCounts) {
	priceSmooth(c.lit[:], p.lit[:])
	priceSmooth(c.of[:], p.of[:])
	priceSmooth(c.ml[:], p.ml[:])

	for l := p.minMatch; l <= maxMatchLen; l++ {
		c, nx := mlCode(l, p.minMatch)
		p.mlPrice[l] = p.ml[c] + uint32(nx)*256
	}
}

func (p *Price) initStatic(src []byte) {
	var c symCounts
	for _, b := range src {
		c.lit[b]++
	}
	p.update(&c)
}

func (p *Price) offPrice(dist int) uint32 {
	c, nx := offCode(dist)
	return p.of[c] + uint32(nx)*256
}

const (
	niceLen = 128
	numPass = 2
)

func findMatch(hits, src []byte, minMatch int) []match {
	base, n := len(hits), len(src)
	buf := make([]byte, base+n)
	copy(buf, hits)
	copy(buf[base:], src)

	chain := HashChain{MaxChain: maxHashChain}
	for i := max(0, base-windowSize); i < base; i++ {
		chain.Insert(buf, i)
	}

	ms := make([]match, n)

	for i := 0; i < n; i++ {
		pos := base + i
		posLen, posOff := chain.longestMatch(buf, pos)
		chain.Insert(buf, pos)
		if posLen < minMatch {
			continue
		}

		ms[i] = match{uint16(posLen), uint32(posOff)}
		if posLen >= niceLen {
			for k := 1; k < posLen; k++ {
				chain.Insert(buf, pos+k)
			}
			i += posLen - 1
		}
	}

	return ms
}

type node struct {
	cost uint32
	match
}

func parseDP(src []byte, ms []match, p *Price) []node {
	n, minMatch := len(src), p.minMatch
	dp := make([]node, n+1)
	for i := 1; i <= n; i++ {
		dp[i].cost = math.MaxUint32
	}

	for i := range n {
		c := dp[i].cost
		if nc := c + p.lit[src[i]]; nc < dp[i+1].cost {
			dp[i+1] = node{cost: nc}
		}

		m := ms[i]
		l := int(m.len)

		if l < minMatch || l > maxMatchLen {
			continue
		}

		base := c + p.offPrice(int(m.off))
		if l < niceLen {
			for j := minMatch; j <= l; j++ {
				if nc := base + p.mlPrice[j]; nc < dp[i+j].cost {
					dp[i+j] = node{nc, match{uint16(j), m.off}}
				}
			}
			continue
		}

		if nc := base + p.mlPrice[l]; nc < dp[i+l].cost {
			dp[i+l] = node{nc, m}
		}
	}
	return dp
}

type token struct {
	start int
	match
}

func traceAndBuild(dp []node, src []byte) Block {
	var tokens []token

	for j := len(src); j > 0; {
		nd := dp[j]
		if nd.len == 0 {
			j--
			continue
		}

		j -= int(nd.len)
		tokens = append(tokens, token{j, nd.match})
	}

	slices.Reverse(tokens)

	var b Block
	pos := 0

	for _, tok := range tokens {
		b.Literals = append(b.Literals, src[pos:tok.start]...)
		b.Seqs = append(b.Seqs, Sequence{
			LitLen:   uint32(tok.start - pos),
			MatchLen: uint32(tok.len),
			Offset:   tok.off,
		})

		pos = tok.start + int(tok.len)
	}

	b.Literals = append(b.Literals, src[pos:]...)
	return b
}

// TODO: need to re eslitmate
const (
	mlDirect    = 16
	numMLCodes  = 28
	numOffCodes = 17
)

// mlCode let the match length of code transform to zstd
func mlCode(l, minMatch int) (code, nExtra int) {
	v := uint32(l - minMatch)
	if v < mlDirect {
		return int(v), 0
	}

	nb := bits.Len32(v)
	return mlDirect + nb - 5, nb - 1
}

// offCode: dist must be >= 1.
func offCode(dist int) (code, nExtra int) {
	nb := bits.Len32(uint32(dist))
	return nb - 1, nb - 1
}

func countSymbols(b Block, minMatch int) (c symCounts) {
	for _, l := range b.Literals {
		c.lit[l]++
	}

	for _, s := range b.Seqs {
		mc, _ := mlCode(int(s.MatchLen), minMatch)
		c.ml[mc]++
		oc, _ := offCode(int(s.Offset))
		c.of[oc]++
	}
	return
}

func compressDP(hits, src []byte, minMatch int) Block {
	if minMatch < 3 || len(src) > blockSize {
		// TODO: avoid the panic
		panic("invalid minMatch")
	}

	if len(src) == 0 {
		return Block{}
	}

	ms := findMatch(hits, src, minMatch)
	p := Price{minMatch: minMatch}
	p.initStatic(src)

	var b Block
	for pass := range numPass {
		dp := parseDP(src, ms, &p)
		b = traceAndBuild(dp, src)
		if pass+1 < numPass {
			c := countSymbols(b, minMatch)
			p.update(&c)
		}
	}
	return b
}
