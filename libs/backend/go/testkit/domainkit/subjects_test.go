package domainkit_test

import (
	"strconv"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
)

// counter is the fixture aggregate of this package: two UPRs with an accepting
// and a refusing branch each, which is what every field of a Projection needs.
type counter struct {
	id    string
	total int
	limit int
}

type bump struct {
	By int
	At int64
}

type reset struct{ At int64 }

type bumpResponse struct {
	Counter string
	Total   int
}

type resetResponse struct{ Counter string }

type bumped struct {
	Counter string
	By      int
	Total   int
	At      int64
}

func (bumped) EventName() string { return "counters.bumped" }

type wasReset struct {
	Counter string
	From    int
	At      int64
}

func (wasReset) EventName() string { return "counters.reset" }

const (
	codeLimitExceeded domain.Code = "counters/limit-exceeded"
	codeAlreadyZero   domain.Code = "counters/already-zero"

	messageLimitExceeded = "the counter would exceed its limit"
	messageAlreadyZero   = "the counter is already at zero"
)

func (c *counter) bump(cmd bump) (domain.Accepted[bumpResponse], *domain.Rejection) {
	attempted := c.total + cmd.By
	if attempted > c.limit {
		return domain.Accepted[bumpResponse]{}, domain.Reject(codeLimitExceeded, messageLimitExceeded,
			domain.Detail{Key: "limit", Value: strconv.Itoa(c.limit)},
			domain.Detail{Key: "attempted", Value: strconv.Itoa(attempted)})
	}
	c.total = attempted
	return domain.Accept(
		bumpResponse{Counter: c.id, Total: c.total},
		bumped{Counter: c.id, By: cmd.By, Total: c.total, At: cmd.At},
	), nil
}

func (c *counter) reset(cmd reset) (domain.Accepted[resetResponse], *domain.Rejection) {
	if c.total == 0 {
		return domain.Accepted[resetResponse]{}, domain.Reject(codeAlreadyZero, messageAlreadyZero)
	}
	from := c.total
	c.total = 0
	return domain.Accept(resetResponse{Counter: c.id}, wasReset{Counter: c.id, From: from, At: cmd.At}), nil
}

func counterState(c *counter) domainkit.Fields {
	return domainkit.Fields{
		"id":    c.id,
		"total": strconv.Itoa(c.total),
		"limit": strconv.Itoa(c.limit),
	}
}

func counterFromState(f domainkit.Fields) *counter {
	total, _ := strconv.Atoi(f["total"])
	limit, _ := strconv.Atoi(f["limit"])
	return &counter{id: f["id"], total: total, limit: limit}
}

func counterEvent(e domain.DomainEvent) (string, domainkit.Fields) {
	switch ev := e.(type) {
	case bumped:
		return ev.EventName(), domainkit.Fields{
			"counter": ev.Counter,
			"by":      strconv.Itoa(ev.By),
			"total":   strconv.Itoa(ev.Total),
			"at":      strconv.FormatInt(ev.At, 10),
		}
	case wasReset:
		return ev.EventName(), domainkit.Fields{
			"counter": ev.Counter,
			"from":    strconv.Itoa(ev.From),
			"at":      strconv.FormatInt(ev.At, 10),
		}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func cloneCounter(c *counter) *counter {
	copied := *c
	return &copied
}

func bumpSubject(cmd bump) domainkit.Subject[*counter, bumpResponse] {
	return domainkit.Subject[*counter, bumpResponse]{
		Decide: func(c *counter) (domain.Accepted[bumpResponse], *domain.Rejection) { return c.bump(cmd) },
		Response: func(r bumpResponse) domainkit.Fields {
			return domainkit.Fields{"counter": r.Counter, "total": strconv.Itoa(r.Total)}
		},
		Event:    counterEvent,
		Snapshot: counterState,
		Clone:    cloneCounter,
	}
}

func resetSubject(cmd reset) domainkit.Subject[*counter, resetResponse] {
	return domainkit.Subject[*counter, resetResponse]{
		Decide:   func(c *counter) (domain.Accepted[resetResponse], *domain.Rejection) { return c.reset(cmd) },
		Response: func(r resetResponse) domainkit.Fields { return domainkit.Fields{"counter": r.Counter} },
		Event:    counterEvent,
		Snapshot: counterState,
		Clone:    cloneCounter,
	}
}
