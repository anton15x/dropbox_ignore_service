package main

import (
	"sync"
	"time"
)

func debounce(f func(), t time.Duration) func() {
	var m sync.Mutex
	called := false
	lastF := f

	return func() {
		m.Lock()
		defer m.Unlock()

		if called {
			lastF = f
			return
		}

		called = true
		f()
		go func() {
			for {
				time.Sleep(t)

				func() {
					m.Lock()
					defer m.Unlock()

					if called {
						lastF()
					} else {
						called = false
						return
					}
				}()
			}
		}()
	}
}
