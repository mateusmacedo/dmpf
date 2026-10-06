package kafka

import (
	"context"
	"errors"
	"sync"
)

var ErrPanicked = errors.New("kafka: a consumer goroutine panicked")

type fault struct {
	mu    sync.Mutex
	err   error
	abort context.CancelFunc
}

func (f *fault) arm(abort context.CancelFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err, f.abort = nil, abort
}

func (f *fault) catch() { f.record(recover()) }

func (f *fault) record(recovered any) {
	if recovered == nil {
		return
	}
	f.mu.Lock()
	if f.err == nil {
		f.err = ErrPanicked
	}
	abort := f.abort
	f.mu.Unlock()
	if abort != nil {
		abort()
	}
}

func (f *fault) result() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}
