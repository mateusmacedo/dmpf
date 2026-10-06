package boot_test

import (
	"os"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/boot"
)

// WHY: the warn of an http:// endpoint is once per process, so it would land in
// whichever test boots first and break the ones that count every record.
func TestMain(m *testing.M) {
	boot.SpendPlaintextExportWarning()
	os.Exit(m.Run())
}
