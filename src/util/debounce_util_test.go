package util_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/anton15x/dropbox_ignore_service/src/util"
	"github.com/stretchr/testify/require"
)

func TestDebounce(t *testing.T) {
	// sleepTime should be big
	const sleepTime = 3 * time.Second

	tests := []struct {
		name string
		f    func(t *testing.T)
	}{
		{
			name: "called_once",
			f: func(t *testing.T) {
				called := make(chan int, 10)
				var sleepWg sync.WaitGroup
				sleepWg.Add(1)
				firstCall := true
				var wg sync.WaitGroup
				wg.Add(1)
				debounced := util.DebounceWithSleepFunc(func(val int) {
					called <- val
				}, func() {
					if firstCall {
						firstCall = false
						sleepWg.Done()
						wg.Wait()
					}
				})

				debounced(1)
				debounced(2)
				debounced(3)
				require.Equal(t, 1, <-called)
				sleepWg.Wait()
				require.Len(t, called, 0)
				wg.Done()
			},
		},
		{
			name: "debounce_called_once_should_not_call_afterwards",
			f: func(t *testing.T) {
				called := 0
				sleepCalled := 0
				ctx, ctxStop := context.WithCancel(context.Background())
				defer ctxStop()

				var wg sync.WaitGroup
				debounced := util.DebounceWithSleepFunc(func(val int) {
					called++
					wg.Done()
				}, func() {
					sleepCalled++
					<-ctx.Done()
					ctx, ctxStop = context.WithCancel(context.Background())
				})

				require.Equal(t, 0, called)

				wg.Add(1)
				debounced(1)
				wg.Wait()
				require.Equal(t, 1, called)

				ctxStop()
				time.Sleep(sleepTime)
				require.Equal(t, 1, called)

				require.Equal(t, 1, sleepCalled)
			},
		},
		{
			name: "debounce_called_twice_should_call_afterwards_once",
			f: func(t *testing.T) {
				called := 0
				sleepCalled := 0
				ctx, ctxStop := context.WithCancel(context.Background())
				defer ctxStop()
				var wg sync.WaitGroup

				debounced := util.DebounceWithSleepFunc(func(val int) {
					called++
					wg.Done()
				}, func() {
					sleepCalled++
					<-ctx.Done()
					ctx, ctxStop = context.WithCancel(context.Background())
				})

				require.Equal(t, 0, called)
				wg.Add(1)
				debounced(1)
				wg.Wait()
				require.Equal(t, 1, called)
				wg.Add(1)
				debounced(2)
				require.Equal(t, 1, called)

				ctxStop()
				wg.Wait()
				require.Equal(t, 2, called)

				ctxStop()
				time.Sleep(sleepTime)
				require.Equal(t, 2, called)

				require.Equal(t, 2, sleepCalled)
			},
		},
		{
			name: "debounce_called_triple_should_call_afterwards_once",
			f: func(t *testing.T) {
				called := 0
				sleepCalled := 0
				ctx, ctxStop := context.WithCancel(context.Background())
				defer ctxStop()
				var wg sync.WaitGroup

				debounced := util.DebounceWithSleepFunc(func(val int) {
					called++
					wg.Done()
				}, func() {
					sleepCalled++
					<-ctx.Done()
					ctx, ctxStop = context.WithCancel(context.Background())
				})

				require.Equal(t, 0, called)
				wg.Add(1)
				debounced(1)
				wg.Wait()
				require.Equal(t, 1, called)
				debounced(2)
				require.Equal(t, 1, called)
				debounced(3)
				require.Equal(t, 1, called)

				wg.Add(1)
				ctxStop()
				wg.Wait()
				require.Equal(t, 2, called)

				ctxStop()
				time.Sleep(sleepTime)
				require.Equal(t, 2, called)

				require.Equal(t, 2, sleepCalled)
			},
		},
		{
			name: "debounce_called_four_times_should_call_afterwards_once",
			f: func(t *testing.T) {
				called := 0
				sleepCalled := 0
				ctx, ctxStop := context.WithCancel(context.Background())
				defer ctxStop()
				var wg sync.WaitGroup

				debounced := util.DebounceWithSleepFunc(func(val int) {
					called++
					wg.Done()
				}, func() {
					sleepCalled++
					<-ctx.Done()
					ctx, ctxStop = context.WithCancel(context.Background())
				})

				require.Equal(t, 0, called)
				wg.Add(1)
				debounced(1)
				wg.Wait()
				require.Equal(t, 1, called)
				debounced(2)
				require.Equal(t, 1, called)
				debounced(3)
				require.Equal(t, 1, called)
				debounced(4)
				require.Equal(t, 1, called)

				wg.Add(1)
				ctxStop()
				wg.Wait()
				require.Equal(t, 2, called)

				ctxStop()
				time.Sleep(sleepTime)
				require.Equal(t, 2, called)

				require.Equal(t, 2, sleepCalled)
			},
		},
	}
	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			test.f(t)
		})
	}
}
