package fsnotify

import (
	"fmt"
	"path/filepath"

	"github.com/rjeczalik/notify"
)

type Watcher struct {
	Events <-chan Event
	Errors <-chan error

	errChan          chan error
	modificationChan chan notify.EventInfo
}

type Event struct {
	Name string
	Op   Op
}

type Op notify.Event

const (
	Create Op = Op(notify.Create)
	Remove Op = Op(notify.Remove)
	Write  Op = Op(notify.Write)
	Rename Op = Op(notify.Rename)

	All Op = Create | Remove | Write | Rename
)

func (e *Op) String() string {
	return notify.Event(*e).String()
}
func (e *Op) Has(h Op) bool {
	return (*e)&h != 0
}

func NewWatcherRecursive(path string) (*Watcher, error) {
	errChan := make(chan error)
	modificationChan := make(chan notify.EventInfo, 1000)

	err := notify.Watch(filepath.Join(path, "..."), modificationChan, notify.Create|notify.Rename|notify.Remove|notify.Write)
	if err != nil {
		return nil, fmt.Errorf("error watching files: %s", err)
	}

	f := make(chan Event, 1000)
	go func() {
		defer close(f)
		for {
			val, ok := <-modificationChan
			if ok {
				f <- Event{
					Name: val.Path(),
					Op:   Op(val.Event()),
				}
			} else {
				break
			}
		}
	}()

	return &Watcher{
		Events: f,
		Errors: errChan,

		errChan:          errChan,
		modificationChan: modificationChan,
	}, nil
}

func (w *Watcher) Close() error {
	notify.Stop(w.modificationChan)
	close(w.errChan)
	return nil
}
