package retry

import (
	"errors"
	"math/rand"
	"time"
)

// Retry is a utility function to retry a function a number of times with exponential backoff
// and jitter. It will return the result of the function if it succeeds, or the last error if
// it fails.
//
// If the error returned inside Retry is a NonRetryableError, it will stop retrying and
// return the original error for later checking.
func Retry[R interface{}](attempts int, baseSleep time.Duration, f func() (R, error)) (R, error) {
	if res, err := f(); err != nil {
		var s NonRetryableError
		if errors.As(err, &s) {
			// Return the original error for later checking
			return res, s.error
		}

		if attempts--; attempts > 0 {
			// Calculate exponential backoff
			// This multiplies the base sleep time by 2 raised to the power of the remaining attempts,
			// which increases the sleep duration exponentially as the number of attempts decreases.
			sleep := baseSleep * (1 << uint(attempts))

			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return Retry[R](attempts, baseSleep, f)
		}
		return res, err
	} else {
		return res, nil
	}
}

// NonRetryableError is a utility type to return an error that will not be retried by Retry
type NonRetryableError struct {
	error
}

// NewNonRetryableError is a utility function to return a NonRetryableError
func NewNonRetryableError(err error) NonRetryableError {
	return NonRetryableError{err}
}
