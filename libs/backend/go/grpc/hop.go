package grpc

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"

	"google.golang.org/grpc/metadata"
)

const idBytes = 16

var (
	correlationFormat = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	localeFormat      = regexp.MustCompile(`^[A-Za-z]{1,8}(-[A-Za-z0-9]{1,8})*$`)
)

// ValidCorrelation bounds what a caller may name the chain: the value travels
// to every outbox row and envelope of the chain, so anything else is replaced.
func ValidCorrelation(correlation string) bool { return correlationFormat.MatchString(correlation) }

// ValidLocale accepts a language tag without parameters: the value crosses to
// gRPC metadata, which refuses anything outside printable ASCII.
func ValidLocale(locale string) bool { return localeFormat.MatchString(locale) }

// NewID is sixteen random bytes in hex. component prefixes the panic of a failed
// entropy source, so each side of the hop keeps the message it always had.
func NewID(component string) string {
	buffer := make([]byte, idBytes)
	if _, err := rand.Read(buffer); err != nil {
		panic(component + ": the operating system's entropy source failed: " + err.Error())
	}
	return hex.EncodeToString(buffer)
}

type MetadataCarrier metadata.MD

func (c MetadataCarrier) Get(key string) string {
	if values := metadata.MD(c).Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c MetadataCarrier) Set(key, value string) { metadata.MD(c).Set(key, value) }

func (c MetadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}
