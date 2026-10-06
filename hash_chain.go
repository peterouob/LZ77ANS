package block

const (
	hashBits = 15
	hashSize = 1 << hashBits
	wMask    = windowSize - 1
)

func hash3(b []byte) uint32 {
	return (uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])) * 2654435761 >> (32 - hashBits)
}

type HashChain struct {
	head [hashSize]uint32
	prev [windowSize]uint32

	HashChainOpt
}

type HashChainOpt struct {
	MaxChain int
}

func (h *HashChain) Insert(buf []byte, cur int) {
	if cur+minMatchLen > len(buf) {
		return
	}

	hash := hash3(buf[cur:])
	h.prev[cur&wMask] = h.head[hash]
	h.head[hash] = uint32(cur + 1)
}

func (h *HashChain) longestMatch(buf []byte, cur int) (bestLen, bestOff int) {
	if cur+minMatchLen > len(buf) {
		return
	}

	limit := max(0, cur-windowSize)
	chain := h.HashChainOpt.MaxChain

	for next := h.head[hash3(buf[cur:])]; next != 0 && chain > 0; chain-- {
		cand := int(next) - 1
		if cand < limit {
			break
		}

		n := 0
		for (cur+n) < len(buf) && n < maxMatchLen && buf[cand+n] == buf[cur+n] {
			n++
		}

		if n > bestLen {
			bestLen, bestOff = n, cur-cand
			if bestLen == maxMatchLen {
				break
			}
		}

		next = h.prev[cand&wMask]
	}
	return
}
