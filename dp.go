package block

import (
	"math"
	"math/bits"
)

type match struct {
	len uint16
	off uint32
}

// TODO: need to calculate again
const (
	numOfCode   = 32
	numMatchLen = 258
)

type PriceReq struct {
	lit []uint32
	of  []uint32
	ml  []uint32
}

type Price struct {
	lit     [256]uint32
	of      [numOfCode]uint32
	ml      [numMatchLen]uint32
	mlPrice []uint32
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

func (p *Price) staticCount(req PriceReq, minMatch int) {
	priceSmooth(req.lit, p.lit[:])
	priceSmooth(req.of, p.of[:])
	priceSmooth(req.ml, p.ml[:])

	if p.mlPrice == nil {
		p.mlPrice = make([]uint32, maxMatchLen+1)
	}

	for l := minMatch; l <= maxMatchLen; l++ {
		c, nx, _ := mlCode(l, minMatch)
		p.mlPrice[l] = p.ml[c] + uint32(nx)*256
	}
}

func (p *Price) initStatic(src []byte, minMatch int) {
	var lit [256]uint32
	var of [numOfCode]uint32
	var ml [numMatchLen]uint32

	for _, b := range src {
		lit[b]++
	}

	parseReq := PriceReq{lit: lit[:], of: of[:], ml: ml[:]}
	p.staticCount(parseReq, minMatch)
}

func (p *Price) offPrice(dist int) uint32 {
	c, nx, _ := offCode(dist)
	return p.of[c] + uint32(nx)*256
}

const (
	niceLen = 128
	numPass = 2
)

func findMatch(hits, src []byte, minMatch int) []match {
	base, n := len(hits), len(src)
	buf := make([]byte, n)
	copy(buf, hits)
	copy(buf[base:], src)

	chain := HashChain{MaxChain: maxHashChain}
	for i := max(0, base-windowSize); i < base; i++ {
		chain.Insert(buf, i)
	}

	ms := make([]match, 0, n)

	for i := range n {
		pos := base + i
		posLen, posOff := chain.longestMatch(buf, pos)
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
	len  uint16
	off  uint32
}

func parseDP(src []byte, ms []match, p *Price, minMatch int) []node {
	n := len(src)
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

		if l < minMatch {
			continue
		}

		base := c + p.offPrice(int(m.off))
		if l < niceLen {
			for j := minMatch; j <= l; j++ {
				if nc := base + p.mlPrice[j]; nc < dp[j+1].cost {
					dp[j+1] = node{
						cost: nc,
						len:  uint16(j),
						off:  m.off,
					}
				}
			}
			continue
		}

		if nc := base + p.mlPrice[l]; nc < dp[i+1].cost {
			dp[i+l] = node{
				cost: nc,
				len:  uint16(l),
				off:  m.off,
			}
		}
	}
	return dp[1:]
}

// TODO: need to re eslitmate
const (
	mlDirect    = 16
	numMLCodes  = 28
	numOffCodes = 17
)

// mlCode let the match length of code transform to zstd
func mlCode(l, minMatch int) (code, nExtra int, extra uint32) {
	v := uint32(l - minMatch)
	if v < mlDirect {
		return int(v), 0, 0
	}

	nb := bits.Len32(v)
	nExtra = nb - 1
	return mlDirect + nb - 5, nExtra, v - (1 << nExtra)
}

func offCode(dist int) (code, nExtra int, extra uint32) {
	d := uint32(dist)
	nb := bits.Len32(d)
	nExtra = nb - 1
	return nb - 1, nExtra, d - (1 << nExtra)
}
