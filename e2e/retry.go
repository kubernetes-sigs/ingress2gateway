/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"time"
)

// Configures the behavior of a retry function.
type retryConfig struct {
	// Maximum number of retry attempts. Must be at least 1.
	maxAttempts int
	// Time to wait between retry attempts.
	delay time.Duration
}

// Returns a default retryConfig.
func defaultRetryConfig() retryConfig {
	return retryConfig{
		maxAttempts: 5,
		delay:       2 * time.Second,
	}
}

// Executes the given function until it succeeds or the maximum number of attempts is reached.
// Respects context cancellation and logs each failed attempt using the provided logger. The
// attemptMsg function is called for each failed attempt to generate a log message.
func retry(
	ctx context.Context,
	log logger,
	cfg retryConfig,
	attemptMsg func(attempt, maxAttempts int, err error) string,
	fn func() error,
) error {
	var err error

	for i := range cfg.maxAttempts {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		err = fn()
		if err == nil {
			return nil
		}

		log.Logf(attemptMsg(i+1, cfg.maxAttempts, err))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(cfg.delay):
		}
	}

	return err
}

// Same as retry but returns generic data on success.
func retryWithData[T any](
	ctx context.Context,
	log logger,
	cfg retryConfig,
	attemptMsg func(attempt, maxAttempts int, err error) string,
	fn func() (T, error),
) (T, error) {
	var result T
	var err error

	for i := range cfg.maxAttempts {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, ctxErr
		}

		result, err = fn()
		if err == nil {
			return result, nil
		}

		log.Logf(attemptMsg(i+1, cfg.maxAttempts, err))

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(cfg.delay):
		}
	}

	return result, err
}
