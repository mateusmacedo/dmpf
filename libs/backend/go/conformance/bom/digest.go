package bom

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/conformance/internal/rule"
)

func (v *validator) checkDigests() error {
	for _, l := range v.doc.located() {
		e := l.entry
		if e.EvidenceURI == "" || e.EvidenceDigest == "" {
			continue
		}
		file, ok := evidencePath(e.EvidenceURI, v.doc.Release)
		if !ok {
			v.add(rule.CodeB005, l.path, e.Identity,
				fmt.Sprintf("evidence_uri %q não aponta para bom/evidence/%s/", e.EvidenceURI, v.doc.Release))
			continue
		}
		raw, err := fs.ReadFile(v.in.Root, file)
		if errors.Is(err, fs.ErrNotExist) {
			v.add(rule.CodeB005, l.path, e.Identity, fmt.Sprintf("arquivo de evidência %s ausente", file))
			continue
		}
		if err != nil {
			return fmt.Errorf("ler %s: %w", file, err)
		}
		sum := sha256.Sum256(raw)
		if want := "sha256:" + hex.EncodeToString(sum[:]); e.EvidenceDigest != want {
			v.add(rule.CodeB005, l.path, e.Identity,
				fmt.Sprintf("evidence_digest %s difere do SHA-256 de %s (%s)", e.EvidenceDigest, file, want))
		}
	}
	return nil
}

// A URI pode apontar para o forge, mas o digest é conferido contra a cópia
// commitada em bom/evidence/<release>/, a única que o validador alcança.
func evidencePath(uri, release string) (string, bool) {
	prefix := "bom/evidence/" + release + "/"
	i := strings.Index(uri, prefix)
	if i < 0 || len(uri) == i+len(prefix) {
		return "", false
	}
	file := uri[i:]
	return file, fs.ValidPath(file)
}
