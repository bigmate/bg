package background

import (
	"time"
)

const (
	delayed paramIdx = 1 + iota
	periodic
)

type paramIdx uint8

type params struct {
	delay    time.Duration
	interval time.Duration
	idx      paramIdx
}

func (p *params) strategy(idx paramIdx) bool {
	return p.idx == idx
}

//Strategy is scheduling strategy
type Strategy func(p *params)

//Delayed execution strategy
func Delayed(d time.Duration) Strategy {
	return func(p *params) {
		p.delay = d
		p.idx = delayed
	}
}

//Periodic periodic execution strategy
func Periodic(d time.Duration) Strategy {
	return func(p *params) {
		p.interval = d
		p.idx = periodic
	}
}
