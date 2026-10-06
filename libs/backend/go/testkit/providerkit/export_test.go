package providerkit

import "time"

func ShortenConcurrentBarrier(d time.Duration) (restore func()) {
	previous := concurrentBarrier
	concurrentBarrier = d
	return func() { concurrentBarrier = previous }
}
