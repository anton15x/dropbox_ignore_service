//go:build !windows

package main

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
	attrs, err := xattr.List(path)
	if err != nil {
		err = handleXattrErr(err)
		return false, err
	}
	found := false
	for _, attr := range attrs {
		if attr == "user.com.dropbox.ignored" {
			found = true
		}
	}
	if !found {
		return false, nil
	}

	b, err := xattr.Get(path, "user.com.dropbox.ignored")
	if err != nil {
		err = handleXattrErr(err)

		// TODO: ENODATA is called, if attribute dos not exist, but that is not exported
		// xattr.list could get removed otherwise
		// if errors.Is(err, xattr.ENODATA) {
		// 	return false, nil
		// }
		return false, err
	}

	return bytes.Equal([]byte("1"), b), nil
}

var implementation *implementationXattr
