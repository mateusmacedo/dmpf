package relay

import (
	"errors"
	"sync"
)

var ErrPanicked = errors.New("relay: the drain loop panicked")

type fault struct {
	once sync.Once
	err  error
}

func (f *fault) catch() {
	if recover() != nil {
		f.once.Do(func() { f.err = ErrPanicked })
	}
}
