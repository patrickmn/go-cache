package cache

import (
	"time"
)

// To avoid redecleration errors, put common code in this file.

const (
	// NoExpiration is for use with functions that take no expiration time.
	NoExpiration time.Duration = -1
	// DefaultExpiration is for use with functions that take an
	// expiration time. Equivalent to passing in the same expiration
	// duration as was given to New() when the cache was
	// created (e.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)
