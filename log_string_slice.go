package main

import "strings"

type logStringSliceStruct struct {
	data     []string
	dataLen  int64
	maxLen   int64
	onUpdate []func()
}

func NewLogStringSlice() *logStringSliceStruct {
	return &logStringSliceStruct{
		// 25MB
		maxLen: 25 * 1024 * 1024,
	}
}

func (l *logStringSliceStruct) Write(p []byte) (n int, err error) {
	pLen := len(p)
	l.data = append(l.data, string(p))
	l.dataLen += int64(pLen)

	for l.dataLen > l.maxLen {
		first := l.data[0]
		l.data = l.data[1:]
		l.dataLen -= int64(len(first))
	}

	for _, f := range l.onUpdate {
		f()
	}

	return pLen, nil
}

func (l *logStringSliceStruct) AddChangeEventListener(f func()) {
	l.onUpdate = append(l.onUpdate, f)
}

func (l *logStringSliceStruct) String() string {
	return strings.Join(l.data, "")
}
