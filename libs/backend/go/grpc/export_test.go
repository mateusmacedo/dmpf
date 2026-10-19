package grpc

import (
	"sync"
	"time"
)

func ResetInsecureClientWarning() { insecureClientWarning = new(sync.Once) }

func ResetInsecureServerWarning() { insecureServerWarning = new(sync.Once) }

func SetShutdownGrace(grace time.Duration) (restore func()) {
	previous := shutdownGrace
	shutdownGrace = grace
	return func() { shutdownGrace = previous }
}

var PeerTrusted = peerTrusted
