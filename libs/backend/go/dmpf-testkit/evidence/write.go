package evidence

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Body struct {
	Tests   []TestResult               `json:"tests"`
	Records map[string]json.RawMessage `json:"records,omitempty"`
}

type Subject struct {
	Header  Header `json:"header"`
	Body    *Body  `json:"body,omitempty"`
	Skipped string `json:"skipped,omitempty"`
}

type IndexEntry struct {
	Subject string `json:"subject"`
	SHA256  string `json:"sha256"`
}

func Encode(s Subject) ([]byte, error) {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Publish builds every file in memory and checks the set against the catalog
// before touching the disk, and never replaces an existing destination: a
// partial or overwritten evidence would still digest cleanly.
func Publish(dest string, catalog []string, subjects []Subject) ([]IndexEntry, error) {
	if _, err := os.Stat(dest); err == nil {
		return nil, fmt.Errorf("%s already exists: publish into a directory that does not exist", dest)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	files := map[string][]byte{}
	index := []IndexEntry{}
	for _, s := range subjects {
		name := s.Header.Subject
		if !isSegment(name) {
			return nil, fmt.Errorf("subject %q is not a single path segment", name)
		}
		if _, dup := files[name+".json"]; dup {
			return nil, fmt.Errorf("subject %s given twice", name)
		}
		raw, err := Encode(s)
		if err != nil {
			return nil, fmt.Errorf("subject %s: %w", name, err)
		}
		files[name+".json"] = raw
		if s.Skipped == "" {
			index = append(index, IndexEntry{Subject: name, SHA256: Digest(raw)})
		}
	}
	want := make([]string, 0, len(catalog))
	for _, c := range catalog {
		want = append(want, c+".json")
	}
	slices.Sort(want)
	if got := slices.Sorted(maps.Keys(files)); !slices.Equal(got, want) {
		return nil, fmt.Errorf("subjects %v differ from the catalog %v", got, want)
	}
	slices.SortFunc(index, func(a, b IndexEntry) int { return cmp.Compare(a.Subject, b.Subject) })
	rawIndex, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return nil, err
	}
	files["index.json"] = append(rawIndex, '\n')

	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(parent, ".dmpf-evidence-")
	if err != nil {
		return nil, err
	}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(tmp, name), raw, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w (partial files stay in %s)", name, err, tmp)
		}
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return nil, fmt.Errorf("%w (partial files stay in %s)", err, tmp)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return nil, fmt.Errorf("publish %s: %w (files stay in %s)", dest, err, tmp)
	}
	return index, nil
}

func ReadRecords(dir, subject string) (map[string]json.RawMessage, error) {
	if !isSegment(subject) {
		return nil, fmt.Errorf("subject %q is not a single path segment", subject)
	}
	entries, err := os.ReadDir(filepath.Join(dir, subject))
	if err != nil {
		return nil, fmt.Errorf("records of %s: %w", subject, err)
	}
	records := map[string]json.RawMessage{}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if e.IsDir() || !ok {
			return nil, fmt.Errorf("records of %s: unexpected entry %s", subject, e.Name())
		}
		raw, err := os.ReadFile(filepath.Join(dir, subject, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("records of %s: %w", subject, err)
		}
		if !json.Valid(raw) {
			return nil, fmt.Errorf("record %s/%s is not valid JSON", subject, name)
		}
		records[name] = raw
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("subject %s recorded nothing", subject)
	}
	return records, nil
}
