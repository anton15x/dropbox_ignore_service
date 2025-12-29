package util

import (
	"sync"
)

type StringSet = Set[string]

func NewStringSet() *Set[string] {
	return NewSet[string]()
}

type Set[T comparable] struct {
	mu       sync.RWMutex
	valueMap map[T]struct{}

	onAdd    []func(T)
	onRemove []func(T)
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{
		valueMap: map[T]struct{}{},
	}
}

func (us *Set[T]) Len() int {
	us.mu.RLock()
	defer us.mu.RUnlock()

	return len(us.valueMap)
}

func (us *Set[T]) Values() []T {
	us.mu.RLock()
	defer us.mu.RUnlock()

	ret := make([]T, 0, len(us.valueMap))
	for key := range us.valueMap {
		ret = append(ret, key)
	}
	return ret
}

func (us *Set[T]) Has(val T) bool {
	us.mu.RLock()
	defer us.mu.RUnlock()

	_, ok := us.valueMap[val]
	return ok
}

func (us *Set[T]) Add(val T) bool {
	us.mu.Lock()
	defer us.mu.Unlock()

	_, ok := us.valueMap[val]
	if ok {
		return false
	}

	us.valueMap[val] = struct{}{}

	for _, onAdd := range us.onAdd {
		onAdd(val)
	}

	return true
}

func (us *Set[T]) Remove(val T) bool {
	us.mu.Lock()
	defer us.mu.Unlock()

	_, ok := us.valueMap[val]
	if !ok {
		return false
	}

	delete(us.valueMap, val)

	for _, onRemove := range us.onRemove {
		onRemove(val)
	}

	return true
}

func (us *Set[T]) RemoveAll() {
	us.mu.Lock()
	defer us.mu.Unlock()

	if len(us.onRemove) > 0 {
		for val := range us.valueMap {
			for _, onRemove := range us.onRemove {
				onRemove(val)
			}
		}
	}

	clear(us.valueMap)
}

func (us *Set[T]) AddAddEventListener(f func(T)) {
	us.mu.Lock()
	defer us.mu.Unlock()

	us.onAdd = append(us.onAdd, f)
}
func (us *Set[T]) AddRemoveEventListener(f func(T)) {
	us.mu.Lock()
	defer us.mu.Unlock()

	us.onRemove = append(us.onRemove, f)
}
func (us *Set[T]) AddChangeEventListener(f func()) {
	f2 := func(T) { f() }
	us.AddAddEventListener(f2)
	us.AddRemoveEventListener(f2)
}
