package util

import (
	"cmp"
	"slices"
	"sync"
)

type SortedStringSet = SortedSet[string]

func NewSortedStringSet() *SortedSet[string] {
	return NewSortedSet[string]()
}

type SortedSet[T cmp.Ordered] struct {
	mu       sync.RWMutex
	values   []T
	valueMap map[T]struct{}

	onAdd    []func(T)
	onRemove []func(T)
}

func NewSortedSet[T cmp.Ordered]() *SortedSet[T] {
	return &SortedSet[T]{
		values:   []T{},
		valueMap: map[T]struct{}{},
	}
}

func (us *SortedSet[T]) Len() int {
	us.mu.RLock()
	defer us.mu.RUnlock()

	return len(us.values)
}

func (us *SortedSet[T]) GetOrEmptyString(i int) T {
	us.mu.RLock()
	defer us.mu.RUnlock()

	if i < len(us.values) {
		return us.values[i]
	}

	var empty T
	return empty
}

func (us *SortedSet[T]) Get(i int) T {
	us.mu.RLock()
	defer us.mu.RUnlock()

	return us.values[i]
}

func (us *SortedSet[T]) Values() []T {
	us.mu.RLock()
	defer us.mu.RUnlock()

	ret := make([]T, len(us.values))
	copy(ret, us.values)
	return ret
}

func (us *SortedSet[T]) Has(val T) bool {
	us.mu.RLock()
	defer us.mu.RUnlock()

	_, ok := us.valueMap[val]
	return ok
}

func (us *SortedSet[T]) Add(val T) bool {
	us.mu.Lock()
	defer us.mu.Unlock()

	_, ok := us.valueMap[val]
	if ok {
		return false
	}

	if len(us.values) == 0 || us.values[len(us.values)-1] < val {
		us.values = append(us.values, val)
	} else {
		i, _ := slices.BinarySearch(us.values, val)
		us.values = slices.Insert(us.values, i, val)
	}
	us.valueMap[val] = struct{}{}

	for _, onAdd := range us.onAdd {
		onAdd(val)
	}

	return true
}

func (us *SortedSet[T]) Remove(val T) bool {
	us.mu.Lock()
	defer us.mu.Unlock()

	_, ok := us.valueMap[val]
	if !ok {
		return false
	}

	i, found := slices.BinarySearch(us.values, val)
	if !found {
		return false
	}
	us.values = slices.Delete(us.values, i, i+1)
	delete(us.valueMap, val)

	for _, onRemove := range us.onRemove {
		onRemove(val)
	}

	return true
}

func (us *SortedSet[T]) RemoveAll() {
	us.mu.Lock()
	defer us.mu.Unlock()

	if len(us.onRemove) > 0 {
		for _, val := range us.values {
			for _, onRemove := range us.onRemove {
				onRemove(val)
			}
		}
	}

	us.values = us.values[:0]
	clear(us.valueMap)
}

func (us *SortedSet[T]) AddAddEventListener(f func(T)) {
	us.mu.Lock()
	defer us.mu.Unlock()

	us.onAdd = append(us.onAdd, f)
}
func (us *SortedSet[T]) AddRemoveEventListener(f func(T)) {
	us.mu.Lock()
	defer us.mu.Unlock()

	us.onRemove = append(us.onRemove, f)
}
func (us *SortedSet[T]) AddChangeEventListener(f func()) {
	f2 := func(T) { f() }
	us.AddAddEventListener(f2)
	us.AddRemoveEventListener(f2)
}
