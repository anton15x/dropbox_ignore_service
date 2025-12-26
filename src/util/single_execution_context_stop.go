package util

import (
	"context"
	"sync"
)

func SingleExecutionContextStop(f func(ctx context.Context)) func() {
	w := SingleExecutionContextStopCtx(f)

	return func() {
		w(context.Background())
	}
}

func SingleExecutionContextStopCtx(f func(ctx context.Context)) func(parent context.Context) {
	var scanCancel context.CancelFunc
	var wg sync.WaitGroup
	var scanMu sync.Mutex

	return func(parent context.Context) {
		scanMu.Lock()
		if scanCancel != nil {
			scanCancel()
			scanCancel = nil
		}
		ctx, cancel := context.WithCancel(parent)
		defer cancel()
		scanCancel = cancel
		wg.Wait()
		wg.Add(1)
		defer wg.Done()
		scanMu.Unlock()

		f(ctx)
	}
}
