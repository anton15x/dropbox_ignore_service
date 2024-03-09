package main

import (
	"slices"
)

type SortedStringSet struct {
	values   []string
	valueMap map[string]interface{}

	onAdd    []func(string)
	onRemove []func(string)
}

func NewSortedStringSet() *SortedStringSet {
	return &SortedStringSet{
		values:   []string{},
		valueMap: map[string]interface{}{},
	}
}

func (us *SortedStringSet) Len() int {
	return len(us.values)
}

func (us *SortedStringSet) GetOrEmptyString(i int) string {
	if i < us.Len() {
		return us.Get(i)
	}

	return ""
}

func (us *SortedStringSet) Get(i int) string {
	return us.values[i]
}

func (us *SortedStringSet) Values() []string {
	ret := make([]string, len(us.values))
	copy(ret, us.values)
	return ret
}

func (us *SortedStringSet) Has(val string) bool {
	_, ok := us.valueMap[val]
	return ok
}

func (us *SortedStringSet) Add(val string) bool {
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

func (us *SortedStringSet) Remove(val string) bool {
	_, ok := us.valueMap[val]
	if !ok {
		return false
	}

	delete(us.valueMap, val)

	us.values = slices.DeleteFunc(us.values, func(s string) bool {
		return s == val
	})

	for _, onRemove := range us.onRemove {
		onRemove(val)
	}

	return true
}

func (us *SortedStringSet) RemoveAll() {
	for _, value := range us.values {
		us.Remove(value)
	}
}

func (us *SortedStringSet) AddAddEventListener(f func(string)) {
	us.onAdd = append(us.onAdd, f)
}
func (us *SortedStringSet) AddRemoveEventListener(f func(string)) {
	us.onRemove = append(us.onRemove, f)
}
func (us *SortedStringSet) AddChangeEventListener(f func()) {
	f2 := func(string) { f() }
	us.AddAddEventListener(f2)
	us.AddRemoveEventListener(f2)
}
