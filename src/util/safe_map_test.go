package util_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/anton15x/dropbox_ignore_service/v2/src/util"
)

func BenchmarkIter(b *testing.B) {
	type ComplexType struct {
		Key        string
		Int64      int64
		Time       time.Time
		Duration   time.Duration
		Int8       int8
		Int32      int32
		Complex64  complex64
		Complex128 complex128
	}

	type Key struct {
		Text string
	}
	type Value struct {
		Text        string
		ComplexType ComplexType
	}
	m := util.NewSafeMap[Key, Value]()

	for i := 0; i < 100; i++ {
		m.Store(Key{Text: strconv.Itoa(i)}, Value{Text: strconv.Itoa(i)})
	}

	b.Run("iter_use_all_values", func(b *testing.B) {
		iter := m.Iter()
		for range b.N {
			for key, value := range iter {
				_, _ = key, value
			}
		}
	})
	b.Run("iter_use_key_only", func(b *testing.B) {
		iter := m.Iter()
		for range b.N {
			for key := range iter {
				_ = key
			}
		}
	})
	b.Run("iter_use_value_only", func(b *testing.B) {
		iter := m.Iter()
		for range b.N {
			for _, value := range iter {
				_ = value
			}
		}
	})

	b.Run("Values", func(b *testing.B) {
		iter := m.Values()
		for range b.N {
			for value := range iter {
				_ = value
			}
		}
	})
	b.Run("Keys", func(b *testing.B) {
		iter := m.Keys()
		for range b.N {
			for key := range iter {
				_ = key
			}
		}
	})
}
