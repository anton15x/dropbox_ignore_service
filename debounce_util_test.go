package main_test

import (
	"context"
	"sync"
	"testing"
	"time"

	main "github.com/anton15x/dropbox_ignore_service"
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
				called := 0
				debounced := main.Debounce(func() {
					called++
				}, time.Hour)

				require.Equal(t, 0, called)
				debounced()
				require.Equal(t, 1, called)
				debounced()
				require.Equal(t, 1, called)
				debounced()
				require.Equal(t, 1, called)
			},
		},
		{
			name: "debounce_called_once_should_not_call_afterwards",
			f: func(t *testing.T) {
				called := 0
				sleepCalled := 0
				ctx, ctxStop := context.WithCancel(context.Background())
				defer ctxStop()

				debounced := main.DebounceWithSleepFunc(func() {
					called++
				}, func() {
					sleepCalled++
					<-ctx.Done()
					ctx, ctxStop = context.WithCancel(context.Background())
				})

				require.Equal(t, 0, called)
				debounced()
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

				debounced := main.DebounceWithSleepFunc(func() {
					called++
					wg.Done()
				}, func() {
					sleepCalled++
					<-ctx.Done()
					ctx, ctxStop = context.WithCancel(context.Background())
				})

				wg.Add(2)
				require.Equal(t, 0, called)
				debounced()
				require.Equal(t, 1, called)
				debounced()
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

				debounced := main.DebounceWithSleepFunc(func() {
					called++
					wg.Done()
				}, func() {
					sleepCalled++
					<-ctx.Done()
					ctx, ctxStop = context.WithCancel(context.Background())
				})

				wg.Add(2)
				require.Equal(t, 0, called)
				debounced()
				require.Equal(t, 1, called)
				debounced()
				require.Equal(t, 1, called)
				debounced()
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
			name: "debounce_called_four_times_should_call_afterwards_once",
			f: func(t *testing.T) {
				called := 0
				sleepCalled := 0
				ctx, ctxStop := context.WithCancel(context.Background())
				defer ctxStop()
				var wg sync.WaitGroup

				debounced := main.DebounceWithSleepFunc(func() {
					called++
					wg.Done()
				}, func() {
					sleepCalled++
					<-ctx.Done()
					ctx, ctxStop = context.WithCancel(context.Background())
				})

				wg.Add(2)
				require.Equal(t, 0, called)
				debounced()
				require.Equal(t, 1, called)
				debounced()
				require.Equal(t, 1, called)
				debounced()
				require.Equal(t, 1, called)
				debounced()
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
	}
	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			test.f(t)
		})
	}
}
