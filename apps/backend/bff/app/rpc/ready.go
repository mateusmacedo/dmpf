package rpc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var ErrNotReady = errors.New("rpc: contexts not ready")

// Readiness maps each context to the channel the edge calls it through.
type Readiness map[string]*grpc.ClientConn

// Check reports every context whose channel has no ready backend before ctx
// ends, so one context down never hides another.
func (r Readiness) Check(ctx context.Context) error {
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		notReady []string
	)
	for name, conn := range r {
		wg.Go(func() {
			if !ready(ctx, conn) {
				mu.Lock()
				notReady = append(notReady, name)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	if len(notReady) == 0 {
		return nil
	}
	slices.Sort(notReady)
	return fmt.Errorf("%w: %s", ErrNotReady, strings.Join(notReady, ", "))
}

// WHY: with the health check of the service config (GRP-13), round_robin only
// reports READY once a backend answers SERVING; asking Health/Check directly
// would hit the method policy, which refuses undeclared methods (GRP-16).
func ready(ctx context.Context, conn *grpc.ClientConn) bool {
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return true
		case connectivity.Shutdown:
			return false
		case connectivity.Idle:
			conn.Connect()
		}
		if !conn.WaitForStateChange(ctx, state) {
			return false
		}
	}
}
