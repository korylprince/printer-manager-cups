package retry

import (
	"math/rand"
	"time"
)

var DefaultStrategy = &Strategy{
	Initial:     1 * time.Second,
	MaxRetries:  5,
	MaxDuration: 10 * time.Second,
	MaxJitter:   time.Second,
}

// Strategy is a retry strategy
type Strategy struct {
	Initial     time.Duration
	MaxRetries  uint
	MaxDuration time.Duration
	MaxJitter   time.Duration
	// ShouldRetryFunc returns an error if err indicates further retries won't be successful.
	ShouldRetryFunc func(err error) error
}

// Retry tries f(), returning an error if MaxTries is exhausted
func (s *Strategy) Retry(f func() error) error {
	tries := 0
	backoff := s.Initial
	for {
		err := f()
		if err == nil {
			return nil
		}

		tries++
		if tries == int(s.MaxRetries) {
			return err
		}

		if backoff > s.MaxDuration {
			backoff = s.MaxDuration
		}

		if s.ShouldRetryFunc != nil {
			if rErr := s.ShouldRetryFunc(err); rErr != nil {
				return rErr
			}
		}

		time.Sleep(backoff + time.Duration(rand.Int63n(int64(s.MaxJitter))))
		backoff *= 2
	}
}
