package c

import (
	"testing"

	"example.test/ws/libs/a"
)

func TestNome(t *testing.T) {
	if a.Nome == "" {
		t.Fatal("vazio")
	}
}
