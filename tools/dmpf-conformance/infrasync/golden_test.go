package infrasync_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/infrasync"
)

func TestRenderWithoutMigratorMatchesTheGolden(t *testing.T) {
	files := render(t, workspace(t))
	golden := map[string]string{}
	err := filepath.WalkDir("testdata/golden", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("testdata/golden", path)
		golden[filepath.ToSlash(rel)] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(golden) != len(files) {
		t.Fatalf("%d arquivos gerados, %d no golden", len(files), len(golden))
	}
	for path, content := range files {
		want, ok := golden[path]
		if !ok {
			t.Errorf("%s gerado sem golden", path)
			continue
		}
		if content != want {
			t.Errorf("%s diverge do golden:\n--- gerado\n%s\n--- golden\n%s", path, content, want)
		}
	}
}

func TestRenderWithoutDatabasesMatchesTheGolden(t *testing.T) {
	root := workspace(t)
	manifests, err := infrasync.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := range manifests {
		manifests[i].Database = nil
	}
	files, err := infrasync.Render(root, manifests)
	if err != nil {
		t.Fatal(err)
	}
	rendered := map[string]string{}
	for _, f := range files {
		rendered[f.Path] = string(f.Content)
	}
	compared := 0
	err = filepath.WalkDir("testdata/golden-zero-db", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("testdata/golden-zero-db", path)
		if got := rendered[filepath.ToSlash(rel)]; got != string(want) {
			t.Errorf("%s diverge do golden sem banco:\n--- gerado\n%s\n--- golden\n%s", rel, got, want)
		}
		compared++
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if compared == 0 {
		t.Fatal("golden sem banco vazio")
	}
}
