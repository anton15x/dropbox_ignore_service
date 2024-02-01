package main

import (
	"slices"
)

type SortedStringSet struct {
	Values   []string
	valueMap map[string]interface{}

	onAdd    []func(string)
	onRemove []func(string)
}

func NewSortedStringSet() *SortedStringSet {
	return &SortedStringSet{
		Values:   []string{},
		valueMap: map[string]interface{}{},
	}
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
	us.Values = append(us.Values, val)

	slices.Sort(us.Values)

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
	for i, cVal := range us.Values {
		if cVal == val {
			us.Values = append(us.Values[:i], us.Values[i+1:]...)
			break
		}
	}
	for _, onRemove := range us.onRemove {
		onRemove(val)
	}

	return true
}

func (us *SortedStringSet) RemoveAll() {
	for _, value := range us.Values {
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
