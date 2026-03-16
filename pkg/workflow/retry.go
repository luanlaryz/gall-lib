package workflow

import "time"

// FixedRetryPolicy retries a step a bounded number of times with a fixed delay.
type FixedRetryPolicy struct {
	MaxRetries int
	Delay      time.Duration
}

// Next returns the next retry delay when attempt is still within bounds.
func (p FixedRetryPolicy) Next(attempt int, err error) (time.Duration, bool) {
	if p.MaxRetries <= 0 || attempt > p.MaxRetries {
		return 0, false
	}
	return p.Delay, true
}
