package main

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

func FyneDoSync(a fyne.App, f func()) {
	// do internal of fyne.Do
	a.Driver().DoFromGoroutine(func() {
		f()
	}, true)
}

func FyneDo(a fyne.App, f func()) {
	// do internal of fyne.Do
	a.Driver().DoFromGoroutine(func() {
		f()
	}, false)
}

func Debounce(f func(), t time.Duration) func() {
	retF := DebounceVal(func(_ struct{}) {
		f()
	}, t)

	return func() {
		retF(struct{}{})
	}
}

func DebounceVal[T any](f func(val T), t time.Duration) func(val T) {
	return DebounceWithSleepFunc(f, func() { time.Sleep(t) })
}

func DebounceWithSleepFunc[T any](f func(val T), sleep func()) func(val T) {
	var m sync.Mutex
	called := false
	var tailingValue T
	needTrailingCall := false

	return func(val T) {
		m.Lock()

		if called {
			tailingValue = val
			needTrailingCall = true
			m.Unlock()
			return
		}

		called = true
		m.Unlock()

		go func() {
			f(val)

			for {
				sleep()

				m.Lock()
				if needTrailingCall {
					localVal := tailingValue
					needTrailingCall = false
					m.Unlock()
					f(localVal)
				} else {
					called = false
					// reset tailingValue the empty value only once after exiting goroutine, to clean up memory
					// probable unnoticeable performance impact instead of doing it every time
					var empty T
					tailingValue = empty
					m.Unlock()
					return
				}
			}
		}()
	}
}
