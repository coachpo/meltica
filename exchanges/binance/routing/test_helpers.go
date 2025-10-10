package routing

import "time"

// SetPrivateKeepAliveInterval overrides the private keepalive ticker interval for tests and
// returns a function that restores the previous value.
func SetPrivateKeepAliveInterval(interval time.Duration) func() {
	old := privateKeepAliveInterval
	privateKeepAliveInterval = interval
	return func() { privateKeepAliveInterval = old }
}
