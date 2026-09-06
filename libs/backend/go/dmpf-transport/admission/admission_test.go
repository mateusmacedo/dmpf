package admission_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/admission"
)

var start = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func controller(t *testing.T, c *clock.Fake, maxKeys int, limits map[string]admission.Limit) *admission.Controller {
	t.Helper()
	tenants, err := metrics.DeclareTenants("acme", "globex")
	if err != nil {
		t.Fatal(err)
	}
	ctrl, err := admission.New(admission.Config{Limits: limits, Tenants: tenants, MaxKeys: maxKeys, Clock: c})
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}
	return ctrl
}

func admit(t *testing.T, ctrl *admission.Controller, route, tenant string, want admission.Reason) func() {
	t.Helper()
	release, reason := ctrl.Admit(route, tenant)
	if reason != want {
		t.Fatalf("Admit(%s, %s) = %s, want %s", route, tenant, reason, want)
	}
	if release == nil {
		t.Fatal("Admit() returned a nil release")
	}
	return release
}

func TestConfigValidate(t *testing.T) {
	c := clock.NewFake(start)
	good := map[string]admission.Limit{"/orders": {PerSecond: 10, Burst: 5, Concurrency: 2}}

	cases := map[string]struct {
		cfg  admission.Config
		want error
	}{
		"no routes":        {admission.Config{Clock: c, MaxKeys: 10}, admission.ErrNoRoutes},
		"no clock":         {admission.Config{Limits: good, MaxKeys: 10}, admission.ErrNoClock},
		"no key ceiling":   {admission.Config{Limits: good, Clock: c}, admission.ErrNoKeyCeiling},
		"zero rate":        {admission.Config{Limits: map[string]admission.Limit{"/x": {Burst: 1, Concurrency: 1}}, Clock: c, MaxKeys: 1}, admission.ErrInvalidLimit},
		"zero burst":       {admission.Config{Limits: map[string]admission.Limit{"/x": {PerSecond: 1, Concurrency: 1}}, Clock: c, MaxKeys: 1}, admission.ErrInvalidLimit},
		"zero concurrency": {admission.Config{Limits: map[string]admission.Limit{"/x": {PerSecond: 1, Burst: 1}}, Clock: c, MaxKeys: 1}, admission.ErrInvalidLimit},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := admission.New(tc.cfg); !errors.Is(err, tc.want) {
				t.Fatalf("New() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestUndeclaredRouteIsRefused(t *testing.T) {
	c := clock.NewFake(start)
	ctrl := controller(t, c, 10, map[string]admission.Limit{"/orders": {PerSecond: 10, Burst: 5, Concurrency: 2}})

	admit(t, ctrl, "/payments", "acme", admission.UndeclaredRoute)()
	if ctrl.Keys() != 0 {
		t.Fatalf("Keys() = %d after an undeclared route, want 0", ctrl.Keys())
	}
}

func TestRateLimitRefillsWithTheClock(t *testing.T) {
	c := clock.NewFake(start)
	ctrl := controller(t, c, 10, map[string]admission.Limit{"/orders": {PerSecond: 2, Burst: 3, Concurrency: 100}})

	for range 3 {
		admit(t, ctrl, "/orders", "acme", admission.Admitted)()
	}
	admit(t, ctrl, "/orders", "acme", admission.RateLimited)()

	c.Advance(500 * time.Millisecond)
	admit(t, ctrl, "/orders", "acme", admission.Admitted)()
	admit(t, ctrl, "/orders", "acme", admission.RateLimited)()

	c.Advance(10 * time.Second)
	for range 3 {
		admit(t, ctrl, "/orders", "acme", admission.Admitted)()
	}
	admit(t, ctrl, "/orders", "acme", admission.RateLimited)()
}

func TestConcurrencyCeilingAndRelease(t *testing.T) {
	c := clock.NewFake(start)
	ctrl := controller(t, c, 10, map[string]admission.Limit{"/orders": {PerSecond: 1000, Burst: 1000, Concurrency: 2}})

	first := admit(t, ctrl, "/orders", "acme", admission.Admitted)
	second := admit(t, ctrl, "/orders", "acme", admission.Admitted)
	admit(t, ctrl, "/orders", "acme", admission.Saturated)()

	first()
	first()
	third := admit(t, ctrl, "/orders", "acme", admission.Admitted)
	admit(t, ctrl, "/orders", "acme", admission.Saturated)()

	second()
	third()
	admit(t, ctrl, "/orders", "acme", admission.Admitted)()
}

func TestTenantsAreIsolatedAndUndeclaredOnesShareOther(t *testing.T) {
	c := clock.NewFake(start)
	ctrl := controller(t, c, 10, map[string]admission.Limit{"/orders": {PerSecond: 1, Burst: 1, Concurrency: 10}})

	admit(t, ctrl, "/orders", "acme", admission.Admitted)()
	admit(t, ctrl, "/orders", "acme", admission.RateLimited)()
	admit(t, ctrl, "/orders", "globex", admission.Admitted)()

	admit(t, ctrl, "/orders", "initech", admission.Admitted)()
	admit(t, ctrl, "/orders", "umbrella", admission.RateLimited)()
	admit(t, ctrl, "/orders", "", admission.RateLimited)()

	if ctrl.Keys() != 3 {
		t.Fatalf("Keys() = %d, want 3 (acme, globex, other)", ctrl.Keys())
	}
}

func TestKeyCeilingEvictsTheIdleAndRefusesWhenNoneIsIdle(t *testing.T) {
	c := clock.NewFake(start)
	limits := map[string]admission.Limit{}
	for _, r := range []string{"/a", "/b", "/c"} {
		limits[r] = admission.Limit{PerSecond: 100, Burst: 100, Concurrency: 5}
	}
	ctrl := controller(t, c, 2, limits)

	holdA := admit(t, ctrl, "/a", "acme", admission.Admitted)
	admit(t, ctrl, "/b", "acme", admission.Admitted)()
	if ctrl.Keys() != 2 {
		t.Fatalf("Keys() = %d, want 2", ctrl.Keys())
	}

	// /b is idle and least recently used: it is evicted for /c.
	admit(t, ctrl, "/c", "acme", admission.Admitted)()
	if ctrl.Keys() != 2 {
		t.Fatalf("Keys() = %d after eviction, want 2", ctrl.Keys())
	}

	// /a is held and /c is now idle; /c is evicted for /b.
	admit(t, ctrl, "/b", "acme", admission.Admitted)()

	// Hold every bucket: nothing is idle, so a new key is refused, not admitted untracked.
	holdB := admit(t, ctrl, "/b", "acme", admission.Admitted)
	admit(t, ctrl, "/c", "acme", admission.Saturated)()

	holdA()
	holdB()
	admit(t, ctrl, "/c", "acme", admission.Admitted)()
}

func TestReleaseIsIdempotent(t *testing.T) {
	c := clock.NewFake(start)
	ctrl := controller(t, c, 10, map[string]admission.Limit{"/orders": {PerSecond: 1000, Burst: 1000, Concurrency: 1}})

	release := admit(t, ctrl, "/orders", "acme", admission.Admitted)
	release()
	release()
	release()

	first := admit(t, ctrl, "/orders", "acme", admission.Admitted)
	admit(t, ctrl, "/orders", "acme", admission.Saturated)()
	first()
}

func TestConcurrentAdmissionNeverExceedsTheCeiling(t *testing.T) {
	c := clock.NewFake(start)
	const ceiling = 4
	ctrl := controller(t, c, 10, map[string]admission.Limit{"/orders": {PerSecond: 1e6, Burst: 1e6, Concurrency: ceiling}})

	var mu sync.Mutex
	inflight, peak := 0, 0
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				release, reason := ctrl.Admit("/orders", "acme")
				if reason != admission.Admitted {
					release()
					continue
				}
				mu.Lock()
				inflight++
				peak = max(peak, inflight)
				mu.Unlock()

				mu.Lock()
				inflight--
				mu.Unlock()
				release()
			}
		}()
	}
	wg.Wait()

	if peak > ceiling {
		t.Fatalf("peak in-flight = %d, exceeds the ceiling %d", peak, ceiling)
	}
	if peak == 0 {
		t.Fatal("nothing was admitted: the test proved nothing")
	}
}
