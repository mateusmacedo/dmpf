package s3

import (
	"context"
	"errors"
	"sync"
	"testing"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func tenantCtx(t *testing.T, tenant ports.TenantID) context.Context {
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

func TestNewRefusesAnIncompleteConfiguration(t *testing.T) {
	cfg := validConfig()
	cfg.Declaration.Scope = ""
	store, err := New(cfg)
	if !errors.Is(err, ports.ErrScopeUndeclared) || store != nil {
		t.Fatalf("New() = %v, %v; want nil, ErrScopeUndeclared", store, err)
	}
}

func TestReadyConfirmsVersioningAndLock(t *testing.T) {
	api := &fakeAPI{}
	store := newTestStore(t, unitConfig(), api)
	if err := store.Ready(context.Background()); err != nil {
		t.Fatalf("Ready() = %v", err)
	}
	if got := api.called(); len(got) != 2 || got[0] != "GetBucketVersioning" || got[1] != "GetObjectLockConfiguration" {
		t.Fatalf("calls = %v", got)
	}
}

func TestReadyRefusesABucketThatIsNotImmutable(t *testing.T) {
	cases := map[string]*fakeAPI{
		"versioning suspended": {versioning: func() (*awss3.GetBucketVersioningOutput, error) {
			return &awss3.GetBucketVersioningOutput{Status: types.BucketVersioningStatusSuspended}, nil
		}},
		"never versioned": {versioning: func() (*awss3.GetBucketVersioningOutput, error) {
			return &awss3.GetBucketVersioningOutput{}, nil
		}},
		"lock not configured": {lock: func() (*awss3.GetObjectLockConfigurationOutput, error) {
			return nil, &smithy.GenericAPIError{Code: "ObjectLockConfigurationNotFoundError", Fault: smithy.FaultClient}
		}},
		"default retention": {lock: func() (*awss3.GetObjectLockConfigurationOutput, error) {
			return &awss3.GetObjectLockConfigurationOutput{ObjectLockConfiguration: &types.ObjectLockConfiguration{
				ObjectLockEnabled: types.ObjectLockEnabledEnabled,
				Rule:              &types.ObjectLockRule{DefaultRetention: &types.DefaultRetention{Mode: types.ObjectLockRetentionModeCompliance}},
			}}, nil
		}},
		"lock disabled": {lock: func() (*awss3.GetObjectLockConfigurationOutput, error) {
			return &awss3.GetObjectLockConfigurationOutput{ObjectLockConfiguration: &types.ObjectLockConfiguration{}}, nil
		}},
	}
	for name, api := range cases {
		t.Run(name, func(t *testing.T) {
			store := newTestStore(t, unitConfig(), api)
			if err := store.Ready(context.Background()); !errors.Is(err, ErrBucketNotImmutable) {
				t.Fatalf("Ready() = %v, want ErrBucketNotImmutable", err)
			}
			if _, err := store.Head(tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc", Version: "v1"}); !errors.Is(err, ErrNotReady) {
				t.Fatalf("Head() after a refused Ready = %v, want ErrNotReady", err)
			}
		})
	}
}

func TestOperationsRefuseBeforeReadyWithoutIO(t *testing.T) {
	api := &fakeAPI{}
	store := newTestStore(t, unitConfig(), api)
	ctx := tenantCtx(t, "acme")
	ref := ports.ObjectRef{Key: "doc", Version: "v1"}
	body := []byte("evidence")
	digest, _ := digestOf(body)
	if _, err := store.Put(ctx, ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Put() = %v, want ErrNotReady", err)
	}
	if _, err := store.Get(ctx, ref); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Get() = %v, want ErrNotReady", err)
	}
	if _, err := store.Purge(ctx, ref); !errors.Is(err, ErrBucketNotImmutable) {
		t.Fatalf("Purge() = %v, want a refusal of the ErrBucketNotImmutable kind", err)
	}
	if got := api.called(); len(got) != 0 {
		t.Fatalf("calls before Ready = %v, want none", got)
	}
}

func TestPureRefusalsComeBeforeTheReadinessGate(t *testing.T) {
	store := newTestStore(t, unitConfig(), &fakeAPI{})
	ctx := tenantCtx(t, "acme")
	if _, err := store.Get(ctx, ports.ObjectRef{Key: "doc"}); !errors.Is(err, ports.ErrVersionRequired) {
		t.Fatalf("Get(no version) = %v, want ErrVersionRequired before the gate", err)
	}
	if _, err := store.Head(context.Background(), ports.ObjectRef{Key: "doc", Version: "v1"}); !errors.Is(err, ports.ErrContextAbsent) {
		t.Fatalf("Head(no carrier) = %v, want ErrContextAbsent before the gate", err)
	}
	if _, err := store.Put(ctx, ports.PutRequest{Key: "doc", Body: []byte("x"), ContentSHA256: "bad"}); !errors.Is(err, ports.ErrIntegrity) {
		t.Fatalf("Put(bad hash) = %v, want ErrIntegrity before the gate", err)
	}
}

func TestReadyMayBeRepeatedAfterAFailure(t *testing.T) {
	failed := false
	api := &fakeAPI{versioning: func() (*awss3.GetBucketVersioningOutput, error) {
		if !failed {
			failed = true
			return nil, &smithy.GenericAPIError{Code: "ServiceUnavailable", Fault: smithy.FaultServer}
		}
		return &awss3.GetBucketVersioningOutput{Status: types.BucketVersioningStatusEnabled}, nil
	}}
	store := newTestStore(t, unitConfig(), api)
	if err := store.Ready(context.Background()); err == nil || errors.Is(err, ErrBucketNotImmutable) {
		t.Fatalf("Ready() = %v, want the transient failure", err)
	}
	if err := store.Ready(context.Background()); err != nil {
		t.Fatalf("Ready() after the failure = %v, want nil", err)
	}
}

func TestReadyHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	api := &fakeAPI{versioning: func() (*awss3.GetBucketVersioningOutput, error) {
		return nil, context.Canceled
	}}
	store := newTestStore(t, unitConfig(), api)
	if err := store.Ready(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ready(cancelled) = %v, want context.Canceled", err)
	}
	if _, err := store.Head(tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc", Version: "v1"}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Head() after a cancelled Ready = %v, want ErrNotReady", err)
	}
}

func TestReadyIsSafeConcurrently(t *testing.T) {
	store := newTestStore(t, unitConfig(), &fakeAPI{})
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Go(func() { errs <- store.Ready(context.Background()) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Ready() = %v", err)
		}
	}
	if err := store.confirmed(); err != nil {
		t.Fatalf("confirmed() = %v after concurrent Ready", err)
	}
}
