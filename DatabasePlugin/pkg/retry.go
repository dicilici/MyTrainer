package pkg

import (
	"context"
	"time"
)

const (
	DefaultRetries  = 5
	DefaultInterval = 5 * time.Second
)

// Retry 调用 fn，err != nil 时最多重试 retries 次（每次间隔 interval），受 ctx 取消控制。
func Retry[T any](ctx context.Context, retries int, interval time.Duration, fn func() (T, error)) (T, error) {
	var zero T
	var result T
	var err error
	for i := 0; i <= retries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(interval):
			}
		}
		result, err = fn()
		if err == nil {
			return result, nil
		}
	}
	return zero, err
}
