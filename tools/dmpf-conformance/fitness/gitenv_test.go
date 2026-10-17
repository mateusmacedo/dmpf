package fitness_test

import (
	"os"
	"strings"
	"testing"
)

// Num hook o git exporta GIT_DIR e GIT_INDEX_FILE; herdados, o git dos fixtures
// grava no repositório real em vez do temporário (pre-push do Lefthook numa worktree).
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		if nome, _, _ := strings.Cut(kv, "="); strings.HasPrefix(nome, "GIT_") {
			_ = os.Unsetenv(nome)
		}
	}
	os.Exit(m.Run())
}
