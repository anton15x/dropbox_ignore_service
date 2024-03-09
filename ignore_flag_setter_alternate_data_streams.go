//go:build windows

package main

import (
	"bytes"
	"os"
)

type implementationAlternateDataStreams struct{}

func (*implementationAlternateDataStreams) SetFlag(path string) error {
	return os.WriteFile(path+":com.dropbox.ignored", []byte("1"), os.ModePerm)
}
func (*implementationAlternateDataStreams) RemoveFlag(path string) error {
	err := os.Remove(path + ":com.dropbox.ignored")
	if err != nil {
		if os.IsNotExist(err) {
			// flag not exists => ignore that error
			return nil
		}
		return err
	}
	return nil
}
func (*implementationAlternateDataStreams) HasFlag(path string) (bool, error) {
	b, err := os.ReadFile(path + ":com.dropbox.ignored")
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(b, []byte("1")), nil
}

var implementation *implementationAlternateDataStreams
