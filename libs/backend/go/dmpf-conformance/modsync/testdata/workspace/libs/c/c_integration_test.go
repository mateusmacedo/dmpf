//go:build integration

package c

import (
	"testing"

	"example.test/ws/libs/b"
)

func TestIntegracao(t *testing.T) {
	if b.Nome == "" {
		t.Fatal("vazio")
	}
}
