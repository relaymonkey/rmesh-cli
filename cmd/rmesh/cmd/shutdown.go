package cmd

import "context"

// shutdownErr returns nil for SIGINT/SIGTERM cancellation so Ctrl+C is not reported as an error.
func shutdownErr(err error) error {
	if err == nil || err == context.Canceled {
		return nil
	}
	return err
}
