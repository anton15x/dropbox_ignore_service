package main

import (
	"cmp"
	"slices"
)

type SortedStringSet = SortedSet[string]

func NewSortedStringSet() *SortedSet[string] {
	return NewSortedSet[string]()
}

type SortedSet[T cmp.Ordered] struct {
	values   []T
	valueMap map[T]interface{}

	onAdd    []func(T)
	onRemove []func(T)
}

func NewSortedSet[T cmp.Ordered]() *SortedSet[T] {
	return &SortedSet[T]{
		values:   []T{},
		valueMap: map[T]interface{}{},
	}
}

func (us *SortedSet[T]) Len() int {
	return len(us.values)
}

func (us *SortedSet[T]) GetOrEmptyString(i int) T {
	if i < us.Len() {
		return us.Get(i)
	}

	var empty T
	return empty
}

func (us *SortedSet[T]) Get(i int) T {
	return us.values[i]
}

func (us *SortedSet[T]) Values() []T {
	ret := make([]T, len(us.values))
	copy(ret, us.values)
	return ret
}

func (us *SortedSet[T]) Has(val T) bool {
	_, ok := us.valueMap[val]
	return ok
}

func (us *SortedSet[T]) Add(val T) bool {
	_, ok := us.valueMap[val]
	if ok {
		return false
	}

	us.valueMap[val] = nil
	us.values = append(us.values, val)

	slices.Sort(us.values)

	for _, onAdd := range us.onAdd {
		onAdd(val)
	}

	return true
}

func (us *SortedSet[T]) Remove(val T) bool {
	_, ok := us.valueMap[val]
	if !ok {
		return false
	}

	delete(us.valueMap, val)

	us.values = slices.DeleteFunc(us.values, func(s T) bool {
		return s == val
	})

	for _, onRemove := range us.onRemove {
		onRemove(val)
	}

	return true
}

func (us *SortedSet[T]) RemoveAll() {
	for _, value := range us.values {
		us.Remove(value)
	}
}

func (us *SortedSet[T]) AddAddEventListener(f func(T)) {
	us.onAdd = append(us.onAdd, f)
}
func (us *SortedSet[T]) AddRemoveEventListener(f func(T)) {
	us.onRemove = append(us.onRemove, f)
}
func (us *SortedSet[T]) AddChangeEventListener(f func()) {
	f2 := func(T) { f() }
	us.AddAddEventListener(f2)
	us.AddRemoveEventListener(f2)
}
