//go:build !windows

package attr

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/pkg/xattr"
)

func handleXattrErr(err error) error {
	if err != nil {
		eErr, ok := err.(*xattr.Error)
		if ok {
			err = fmt.Errorf("error stringified: %s", eErr.Error())
		}
	}
	return err
}

var implementation *implementationXattr

type implementationXattr struct {
}

func (*implementationXattr) SetFlag(path string) error {
	if !xattr.XATTR_SUPPORTED {
		return fmt.Errorf("xattr not supported")
	}
	return handleXattrErr(xattr.Set(path, "user.com.dropbox.ignored", []byte("1")))
}
func (*implementationXattr) RemoveFlag(path string) error {
	if !xattr.XATTR_SUPPORTED {
		return fmt.Errorf("xattr not supported")
	}
	err := xattr.Remove(path, "user.com.dropbox.ignored")
	if errors.Is(err, xattr.ENOATTR) {
		return nil
	}

	return handleXattrErr(err)
}

func (*implementationXattr) HasFlag(path string) (bool, error) {
	if !xattr.XATTR_SUPPORTED {
		return false, fmt.Errorf("xattr not supported")
	}

	b, err := xattr.Get(path, "user.com.dropbox.ignored")
	if err != nil {
		if errors.Is(err, xattr.ENOATTR) {
			return false, nil
		}

		return false, handleXattrErr(err)
	}

	return bytes.Equal([]byte("1"), b), nil
}
