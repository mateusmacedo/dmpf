package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var ErrInvalidExternalBudget = errors.New("app: invalid external budget")

// MinExternalMargin covers the gap between the ledger Begin and the start of
// the Do deadline, which neither Route.Limit nor Settle accounts for.
const MinExternalMargin = time.Second

type ExternalBudget struct {
	Route      deadline.Budget
	Settle     time.Duration
	Processing time.Duration
	Margin     time.Duration
}

func (b ExternalBudget) Validate() error {
	switch {
	case b.Route.Limit <= 0:
		return fmt.Errorf("%w: route limit %v", ErrInvalidExternalBudget, b.Route.Limit)
	case b.Settle <= 0:
		return fmt.Errorf("%w: settle %v", ErrInvalidExternalBudget, b.Settle)
	case b.Processing <= 0:
		return fmt.Errorf("%w: processing %v", ErrInvalidExternalBudget, b.Processing)
	case b.Margin < MinExternalMargin:
		return fmt.Errorf("%w: margin %v below %v", ErrInvalidExternalBudget, b.Margin, MinExternalMargin)
	case b.Route.Limit >= b.Processing || b.Settle >= b.Processing-b.Route.Limit:
		return fmt.Errorf("%w: route limit %v plus settle %v does not fit processing %v",
			ErrInvalidExternalBudget, b.Route.Limit, b.Settle, b.Processing)
	case b.Margin > math.MaxInt64-b.Route.Limit-b.Settle:
		return fmt.Errorf("%w: stale cut of route limit %v, settle %v and margin %v overflows",
			ErrInvalidExternalBudget, b.Route.Limit, b.Settle, b.Margin)
	}
	return nil
}

func (b ExternalBudget) StaleAfter() time.Duration {
	if b.Margin > math.MaxInt64-b.Route.Limit-b.Settle {
		return math.MaxInt64
	}
	return b.Route.Limit + b.Settle + b.Margin
}

func (b ExternalBudget) SettleContext() func(context.Context) (context.Context, context.CancelFunc) {
	return func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, b.Settle)
	}
}
