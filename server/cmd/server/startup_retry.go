package main

import (
	"log"
	"time"
)

// retryStartup calls open until it succeeds or attempts run out, waiting delay
// between tries. Docker ignores depends_on when the daemon or the host
// restarts, so the server can start before MariaDB accepts connections. Waiting
// here avoids a crash and restart loop on every reboot.
func retryStartup[T any](what string, attempts int, delay time.Duration, sleep func(time.Duration), open func() (T, error)) (T, error) {
	var result T
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		result, err = open()
		if err == nil {
			if attempt > 1 {
				log.Printf("%s: ready after %d attempts", what, attempt)
			}
			return result, nil
		}
		if attempt < attempts {
			log.Printf("%s: not ready yet (attempt %d of %d): %v", what, attempt, attempts, err)
			sleep(delay)
		}
	}
	return result, err
}
