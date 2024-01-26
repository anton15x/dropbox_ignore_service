package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"

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

func init() {
	if len(implementations) == 0 {
		panic(fmt.Errorf("xattr not supported"))
	}
}

func SetDropboxIgnoreFlag(path string) error {
	if len(implementations) == 0 {
		return fmt.Errorf("no xattr implementation for os")
	}

	var retErr error
	for _, i := range implementations {
		err := i.SetFlag(path)
		if err == nil {
			return nil
		}

		if retErr == nil {
			retErr = err
		}
	}

	return retErr
}

func RemoveDropboxIgnoreFlag(path string) error {
	if len(implementations) == 0 {
		return fmt.Errorf("no xattr implementation for os")
	}

	var retErr error
	for _, i := range implementations {
		err := i.RemoveFlag(path)
		if err == nil {
			return nil
		}

		if retErr == nil {
			retErr = err
		}
	}

	return retErr
}

func HasDropboxIgnoreFlag(path string) (bool, error) {
	if len(implementations) == 0 {
		return false, fmt.Errorf("no xattr implementation for os")
	}

	var retErr error
	for _, i := range implementations {
		hasFlag, err := i.HasFlag(path)
		if err == nil {
			return hasFlag, nil
		}

		if retErr == nil {
			retErr = err
		}
	}

	return false, retErr
}

type Implementation struct {
	SetFlag    func(path string) error
	RemoveFlag func(path string) error
	HasFlag    func(path string) (bool, error)
	runtimeOS  []string
}

var implementations = []Implementation{
	{
		SetFlag: func(path string) error {
			return os.WriteFile(path+":com.dropbox.ignored", []byte("1"), os.ModePerm)
		},
		RemoveFlag: func(path string) error {
			err := os.Remove(path + ":com.dropbox.ignored")
			if err != nil {
				if os.IsNotExist(err) {
					// flag not exists => ignore that error
					return nil
				}
				return err
			}
			return nil
		},
		HasFlag: func(path string) (bool, error) {
			b, err := os.ReadFile(path + ":com.dropbox.ignored")
			if err != nil {
				if os.IsNotExist(err) {
					return false, nil
				}
				return false, err
			}
			return bytes.Equal(b, []byte("1")), nil
		},
		runtimeOS: []string{"windows"},
	},
	{
		SetFlag: func(path string) error {
			if !xattr.XATTR_SUPPORTED {
				return fmt.Errorf("xattr not supported")
			}
			return handleXattrErr(xattr.Set(path, "user.com.dropbox.ignored", []byte("1")))
		},
		RemoveFlag: func(path string) error {
			if !xattr.XATTR_SUPPORTED {
				return fmt.Errorf("xattr not supported")
			}
			err := xattr.Remove(path, "user.com.dropbox.ignored")
			if errors.Is(err, xattr.ENOATTR) {
				return nil
			}

			return handleXattrErr(err)
		},
		HasFlag: func(path string) (bool, error) {
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
		},
		runtimeOS: func() []string {
			ret := []string{}
			if xattr.XATTR_SUPPORTED {
				ret = append(ret, runtime.GOOS)
			}
			return ret
		}(),
	},
}

func init() {
	implementations = slices.DeleteFunc(implementations, func(i Implementation) bool {
		return !slices.Contains(i.runtimeOS, runtime.GOOS)
	})
}
