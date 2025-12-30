//go:build windows

package attr

import (
	"errors"
	"io"
	"os"
)

var implementation *implementationAlternateDataStreams

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
	f, err := os.Open(path + ":com.dropbox.ignored")
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	var b [2]byte
	read, err := f.Read(b[:])
	if err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, err
	}
	return read == 1 && b[0] == '1', nil
}
