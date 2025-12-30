package util

import (
	"strings"
	"sync"
)

type LogStringSliceStruct struct {
	mu sync.RWMutex

	data     []string
	dataLen  int64
	maxLen   int64
	onUpdate []func()
}

func NewLogStringSlice() *LogStringSliceStruct {
	return &LogStringSliceStruct{
		// 25MB
		maxLen: 25 * 1024 * 1024,
	}
}

func (l *LogStringSliceStruct) Write(p []byte) (n int, err error) {
	pLen := len(p)

	l.mu.Lock()
	l.data = append(l.data, string(p))
	l.dataLen += int64(pLen)
	for l.dataLen > l.maxLen {
		first := l.data[0]
		l.data = l.data[1:]
		l.dataLen -= int64(len(first))
	}
	l.mu.Unlock()

	l.mu.RLock()
	for _, f := range l.onUpdate {
		f()
	}
	l.mu.RUnlock()

	return pLen, nil
}

func (l *LogStringSliceStruct) AddChangeEventListener(f func()) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.onUpdate = append(l.onUpdate, f)
}

func (l *LogStringSliceStruct) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return len(l.data)
}

func (l *LogStringSliceStruct) GetOrEmptyString(i int) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if i >= len(l.data) {
		return ""
	}

	return l.data[i]
}

func (l *LogStringSliceStruct) String() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return strings.Join(l.data, "")
}
