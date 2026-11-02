package worm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
)

const second = ports.Instant(1_000_000_000)

func carrier(t *testing.T, tenant ports.TenantID) context.Context {
	t.Helper()
	spec := ports.ExecutionContextSpec{
		RequestID: "r-test", CorrelationID: "c-test", TraceContext: "t-test",
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	}
	if tenant != "" {
		spec.Tenant = &tenant
	}
	execution, err := ports.NewExecutionContext(spec)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

func TestPhysical(t *testing.T) {
	cases := map[string]struct {
		scope  ports.Scope
		tenant ports.TenantID
		bare   bool
		want   string
		err    error
	}{
		"tenant prefix":                  {scope: ports.ScopeTenant, tenant: "acme", want: "tenant/acme/doc"},
		"tenant named platform":          {scope: ports.ScopeTenant, tenant: "platform", want: "tenant/platform/doc"},
		"tenant store without tenant":    {scope: ports.ScopeTenant, err: worm.ErrTenantUnresolved},
		"tenant with a slash":            {scope: ports.ScopeTenant, tenant: "a/b", err: worm.ErrTenantUnresolved},
		"tenant that is a dot":           {scope: ports.ScopeTenant, tenant: ".", err: worm.ErrTenantUnresolved},
		"tenant that is two dots":        {scope: ports.ScopeTenant, tenant: "..", err: worm.ErrTenantUnresolved},
		"platform prefix":                {scope: ports.ScopePlatform, want: "platform/doc"},
		"platform store with a tenant":   {scope: ports.ScopePlatform, tenant: "acme", err: ports.ErrScopeMismatch},
		"no execution context":           {scope: ports.ScopeTenant, bare: true, err: ports.ErrContextAbsent},
		"platform without execution ctx": {scope: ports.ScopePlatform, bare: true, err: ports.ErrContextAbsent},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if !tc.bare {
				ctx = carrier(t, tc.tenant)
			}
			got, err := worm.Physical(ctx, tc.scope, "doc")
			if tc.err != nil {
				if !errors.Is(err, tc.err) || got != "" {
					t.Fatalf("Physical() = %q, %v; want \"\", %v", got, err, tc.err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("Physical() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestPhysicalHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(carrier(t, "acme"))
	cancel()
	if _, err := worm.Physical(ctx, ports.ScopeTenant, "doc"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Physical(cancelled) = %v, want context.Canceled", err)
	}
}

func TestLocateRefusesInOrder(t *testing.T) {
	if _, err := worm.Locate(context.Background(), ports.ScopeTenant, ports.ObjectRef{Key: "../x"}); !errors.Is(err, ports.ErrVersionRequired) {
		t.Fatalf("Locate(no version) = %v, want ErrVersionRequired first", err)
	}
	if _, err := worm.Locate(context.Background(), ports.ScopeTenant, ports.ObjectRef{Key: "../x", Version: "v"}); !errors.Is(err, ports.ErrObjectKeyInvalid) {
		t.Fatalf("Locate(bad key) = %v, want ErrObjectKeyInvalid before the carrier", err)
	}
	got, err := worm.Locate(carrier(t, "acme"), ports.ScopeTenant, ports.ObjectRef{Key: "doc", Version: "v"})
	if err != nil || got != "tenant/acme/doc" {
		t.Fatalf("Locate() = %q, %v", got, err)
	}
}

func TestWholeSeconds(t *testing.T) {
	got := worm.WholeSeconds(ports.Retention{Mode: ports.RetentionCompliance, Until: 10*second + 999})
	if got.Until != 10*second || got.Mode != ports.RetentionCompliance {
		t.Fatalf("WholeSeconds() = %+v", got)
	}
}

func TestCheckRetention(t *testing.T) {
	ceiling := &ports.RetentionCeiling{Span: ports.RetentionSpan(4 * second), Origin: "test"}
	now := 100 * second
	cases := map[string]struct {
		ceiling *ports.RetentionCeiling
		r       ports.Retention
		want    error
	}{
		"inside the ceiling":    {ceiling, ports.Retention{Mode: ports.RetentionCompliance, Until: now + second}, nil},
		"at the ceiling":        {ceiling, ports.Retention{Mode: ports.RetentionGovernance, Until: now + 4*second}, nil},
		"above the ceiling":     {ceiling, ports.Retention{Mode: ports.RetentionCompliance, Until: now + 5*second}, ports.ErrRetentionAboveCeiling},
		"expired":               {ceiling, ports.Retention{Mode: ports.RetentionCompliance, Until: now}, ports.ErrRetentionUnbounded},
		"absent":                {ceiling, ports.Retention{Mode: ports.RetentionCompliance}, ports.ErrRetentionUnbounded},
		"unknown mode":          {ceiling, ports.Retention{Mode: "LEGAL", Until: now + second}, ports.ErrRetentionModeUnknown},
		"store without ceiling": {nil, ports.Retention{Mode: ports.RetentionCompliance, Until: now + second}, ports.ErrRetentionCeilingUndeclared},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := worm.CheckRetention(now, tc.ceiling, tc.r)
			if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("CheckRetention() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCheckChange(t *testing.T) {
	now := 100 * second
	compliance := &ports.Retention{Mode: ports.RetentionCompliance, Until: now + 2*second}
	governance := &ports.Retention{Mode: ports.RetentionGovernance, Until: now + 2*second}
	expired := &ports.Retention{Mode: ports.RetentionCompliance, Until: now - second}
	cases := map[string]struct {
		current *ports.Retention
		to      ports.Retention
		want    error
	}{
		"free version":               {nil, ports.Retention{Mode: ports.RetentionGovernance, Until: now + second}, nil},
		"expired version is free":    {expired, ports.Retention{Mode: ports.RetentionGovernance, Until: now + second}, nil},
		"extend compliance":          {compliance, ports.Retention{Mode: ports.RetentionCompliance, Until: now + 3*second}, nil},
		"same date, same mode":       {compliance, ports.Retention{Mode: ports.RetentionCompliance, Until: now + 2*second}, ports.ErrRetentionNotExtended},
		"shorten compliance":         {compliance, ports.Retention{Mode: ports.RetentionCompliance, Until: now + second}, ports.ErrRetentionNotExtended},
		"demote compliance":          {compliance, ports.Retention{Mode: ports.RetentionGovernance, Until: now + 3*second}, ports.ErrRetentionModeChange},
		"promote on the same date":   {governance, ports.Retention{Mode: ports.RetentionCompliance, Until: now + 2*second}, nil},
		"promote on an earlier date": {governance, ports.Retention{Mode: ports.RetentionCompliance, Until: now + second}, ports.ErrRetentionNotExtended},
		"shorten governance":         {governance, ports.Retention{Mode: ports.RetentionGovernance, Until: now + second}, ports.ErrRetentionNotExtended},
		"extend governance":          {governance, ports.Retention{Mode: ports.RetentionGovernance, Until: now + 3*second}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := worm.CheckChange(now, tc.current, tc.to)
			if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("CheckChange() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestPurgeable(t *testing.T) {
	now := 100 * second
	cases := map[string]struct {
		current *ports.Retention
		hold    bool
		until   ports.Instant
		want    error
	}{
		"free":             {nil, false, 0, nil},
		"expired":          {&ports.Retention{Mode: ports.RetentionCompliance, Until: now - second}, false, now - second, nil},
		"retained":         {&ports.Retention{Mode: ports.RetentionGovernance, Until: now + second}, false, 0, ports.ErrObjectLocked},
		"held":             {nil, true, 0, ports.ErrObjectLocked},
		"held and expired": {&ports.Retention{Mode: ports.RetentionCompliance, Until: now - second}, true, 0, ports.ErrObjectLocked},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			until, err := worm.Purgeable(now, tc.current, tc.hold)
			if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) || until != tc.until {
				t.Fatalf("Purgeable() = %v, %v; want %v, %v", until, err, tc.until, tc.want)
			}
		})
	}
}
