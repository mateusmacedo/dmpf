package grpc

import (
	"sync"
	"time"
)

func ResetInsecureClientWarning() { insecureClientWarning = new(sync.Once) }

func ResetInsecureServerWarning() { insecureServerWarning = new(sync.Once) }

var CategoryOf = categoryOf

func SetShutdownGrace(grace time.Duration) (restore func()) {
	previous := shutdownGrace
	shutdownGrace = grace
	return func() { shutdownGrace = previous }
}
