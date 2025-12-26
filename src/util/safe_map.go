package util

import (
	"iter"
	"sync"
)

type SafeMap[K, V any] struct {
	m sync.Map
}

func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{}
}

func (m *SafeMap[K, V]) Load(key K) (value V, ok bool) {
	val, ok := m.m.Load(key)
	if !ok {
		var e V
		return e, false
	}

	return val.(V), true
}

func (m *SafeMap[K, V]) Store(key K, value V) {
	m.m.Store(key, value)
}

func (m *SafeMap[K, V]) Clear() {
	m.m.Clear()
}

func (m *SafeMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	val, loaded := m.m.LoadOrStore(key, value)
	if !loaded {
		var e V
		return e, false
	}
	return val.(V), true
}

func (m *SafeMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	val, loaded := m.m.LoadAndDelete(key)
	if !loaded {
		var e V
		return e, false
	}
	return val.(V), true
}

func (m *SafeMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

func (m *SafeMap[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	val, loaded := m.m.Swap(key, value)
	if !loaded {
		var e V
		return e, false
	}
	return val.(V), true
}

func (m *SafeMap[K, V]) CompareAndSwap(key K, old, new V) (swapped bool) {
	return m.m.CompareAndSwap(key, old, new)
}

func (m *SafeMap[K, V]) CompareAndDelete(key K, old V) (deleted bool) {
	return m.m.CompareAndDelete(key, old)
}

func (m *SafeMap[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(key, value any) bool {
		return f(key.(K), value.(V))
	})
}

// custom methods
// Iter iterates only over keys and values (use Keys or Values if only one is needed)
func (m *SafeMap[K, V]) Iter() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.m.Range(func(key, value any) bool {
			return yield(key.(K), value.(V))
		})
	}
}

// Keys iterates only over keys (faster than Iter, if only keys are needed)
func (m *SafeMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		m.m.Range(func(key, value any) bool {
			return yield(key.(K))
		})
	}
}

// Values iterates only over values (faster than Iter, if only values are needed)
func (m *SafeMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		m.m.Range(func(key, value any) bool {
			return yield(value.(V))
		})
	}
}
