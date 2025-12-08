package main

import (
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

func FyneDo(a fyne.App, f func()) {
	// do internal of fyne.Do
	a.Driver().DoFromGoroutine(func() {
		f()
	}, false)
}

func DebounceFyneDo(a fyne.App, f func(), t time.Duration) func() {
	return Debounce(func() {
		FyneDo(a, f)
	}, t)
}

func Debounce(f func(), t time.Duration) func() {
	return DebounceWithSleepFunc(f, func() { time.Sleep(t) })
}

func DebounceWithSleepFunc(f func(), sleep func()) func() {
	var m sync.Mutex
	called := false
	var lastF func()

	return func() {
		m.Lock()
		defer m.Unlock()

		if called {
			lastF = f
			runtime.Gosched()
			return
		}

		called = true
		f()
		go func() {
			for called {
				sleep()

				func() {
					m.Lock()
					defer m.Unlock()

					if lastF != nil {
						fToExecute := lastF
						lastF = nil
						fToExecute()
					} else {
						called = false
					}
				}()
			}
		}()
	}
}
