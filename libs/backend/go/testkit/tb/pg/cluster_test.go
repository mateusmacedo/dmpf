package pg

import (
	"errors"
	"testing"
)

func TestAServerThatIsNotTheTestClusterIsRefused(t *testing.T) {
	if err := requireTestCluster("runtime"); !errors.Is(err, ErrNotTestCluster) {
		t.Fatalf("requireTestCluster(runtime) = %v, want ErrNotTestCluster", err)
	}
	if err := requireTestCluster(TestCluster); err != nil {
		t.Fatalf("requireTestCluster(%s) = %v, want nil", TestCluster, err)
	}
}
