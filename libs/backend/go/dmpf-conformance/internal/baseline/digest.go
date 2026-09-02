package baseline

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
)

// Digest resume o conjunto, para que alterar uma entrada obrigue a recalculá-lo.
//
// A codificação prefixa cada campo com o comprimento em vez de separá-los por
// um byte: separador só distingue conjuntos enquanto nenhum valor o contém, e
// quem abre o PR edita este arquivo. Racional completo em docs/adr/031.
func Digest(entries []Entry) string {
	h := sha256.New()
	escreverUint(h, uint64(len(entries)))
	for _, e := range entries {
		escreverCampo(h, e.Module)
		escreverCampo(h, e.Unit)
		escreverCampo(h, e.Block)
		escreverCampo(h, e.BoundedContext)
		escreverUint(h, uint64(len(e.Membership)))
		for _, m := range e.Membership {
			escreverCampo(h, m)
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func escreverCampo(h hash.Hash, v string) {
	escreverUint(h, uint64(len(v)))
	h.Write([]byte(v))
}

func escreverUint(h hash.Hash, n uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], n)
	h.Write(buf[:])
}
