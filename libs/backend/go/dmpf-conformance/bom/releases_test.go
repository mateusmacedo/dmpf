package bom

import "testing"

func TestLatestReleaseSegueAPrecedenciaDeSemver(t *testing.T) {
	casos := []struct {
		nome     string
		releases []string
		want     string
	}{
		{"numérica, não lexical", []string{"0.9.0", "0.10.0", "0.2.0"}, "0.10.0"},
		{"pré-release antes da final", []string{"1.0.0", "1.0.0-rc.1"}, "1.0.0"},
		{"identificador numérico antes do alfanumérico", []string{"1.0.0-alpha", "1.0.0-1"}, "1.0.0-alpha"},
		{"mais identificadores vêm depois", []string{"1.0.0-rc", "1.0.0-rc.1"}, "1.0.0-rc.1"},
		{"o que não é semver fica de fora", []string{"latest", "0.1.0", "v0.2.0"}, "0.1.0"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, ok := LatestRelease(c.releases)
			if !ok || got != c.want {
				t.Fatalf("LatestRelease(%v) = %q, %v; esperado %q", c.releases, got, ok, c.want)
			}
		})
	}
	if _, ok := LatestRelease([]string{"latest", "x"}); ok {
		t.Fatal("LatestRelease escolheu release sem nenhuma semver")
	}
}
