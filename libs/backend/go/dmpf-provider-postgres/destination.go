package dmpfpostgres

import (
	"fmt"
	"regexp"
)

// Lowercase dot-separated segments and nothing else: an ARN, a URL, a slash
// path or a topic name would be a physical target, and choosing the physical
// target belongs to the relay, never to the application service (BLK-04).
var destinationForm = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)

func checkDestination(destination string) error {
	if !destinationForm.MatchString(destination) {
		return fmt.Errorf("%w: %q", ErrInvalidDestination, destination)
	}
	return nil
}
