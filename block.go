package block

import "errors"

const (
	minMatchLen = 3
	maxMatchLen = 258
	windowSize  = 32 * 1024
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
