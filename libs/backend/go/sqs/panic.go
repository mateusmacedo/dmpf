package sqs

import (
	"context"
	"errors"
	"sync"
)

var ErrPanicked = errors.New("sqs: a consumer goroutine panicked")

type fault struct {
	mu    sync.Mutex
	err   error
	abort context.CancelFunc
}

func (f *fault) catch() {
	if recover() == nil {
		return
	}
	f.mu.Lock()
	if f.err == nil {
		f.err = ErrPanicked
	}
	f.mu.Unlock()
	f.abort()
}

func (f *fault) result() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}
