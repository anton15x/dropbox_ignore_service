package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"

	"github.com/pkg/xattr"
)

func execCommand(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing %s: %s\n%s", cmd.String(), err, string(b))
	}
	return nil
}
func execCommandGetOutput(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)

	var ret bytes.Buffer
	var b bytes.Buffer

	cmd.Stdout = io.MultiWriter(&b, &ret)
	cmd.Stderr = &b

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error executing %s: %s\n%s", cmd.String(), err, b.String())
	}
	return ret.Bytes(), nil
}

/*
func excludeWindows(path string) error {
	return execCommand("powershell.exe", "Set-Content", "-Path", path, "-Stream", "com.dropbox.ignored", "-Value", "1")
}
func excludeLinux(path string) error {
	return execCommand("attr", "-s", "com.dropbox.ignored", "-V", "1", path)
}
func excludeDarwin(path string) error {
	return execCommand("xattr", "-w", "com.dropbox.ignored", "1", path)
}
*/

func isAttrInstalled() (retOk bool, retErr error) {
	f, err := os.CreateTemp("", "attr_testfile")
	if err != nil {
		return false, fmt.Errorf("error creating testfile: %s", err)
	}
	fileName := f.Name()
	defer func() {
		err := os.Remove(fileName)
		if err != nil {
			err = fmt.Errorf("error removing tmp testfile %s: %s", fileName, err)
			if retErr == nil {
				retErr = err
			} else {
				retErr = fmt.Errorf("%s\n%s", retErr, err)
			}
		}
	}()
	defer func() {
		err := f.Close()
		if err != nil {
			err = fmt.Errorf("error closing file: %s", err)
			if retErr == nil {
				retErr = err
			} else {
				retErr = fmt.Errorf("%s\n%s", retErr, err)
			}
		}
	}()

	if runtime.GOOS == "windows" || xattr.XATTR_SUPPORTED {
		return true, nil
	}
	return false, nil
}

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
	ok, err := isAttrInstalled()
	if err == nil && !ok {
		err = fmt.Errorf("xattr not supported")
	}
	if err != nil {
		panic(err)
	}

	// TODO: test xattr and allow other OS to run this
	// if runtime.GOOS != "windows" {
	// 	panic(fmt.Errorf("currently only windows is supported"))
	// }
}

/*
func includeWindows(path string) error {
	return execCommand("powershell.exe", "Clear-Content", "-Path", path, "-Stream", "com.dropbox.ignored")
}

func includeLinux(path string) error {
	return execCommand("attr", "-r", "com.dropbox.ignored", path)
}
func includeDarwin(path string) error {
	return execCommand("xattr", "-d", "com.dropbox.ignored", path)
}
*/

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
				if errors.Is(err, os.ErrNotExist) {
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
				// TOTO: is this possible?
				if errors.Is(err, xattr.ENOATTR) {
					return false, nil
				}
				return false, err
			}

			return bytes.Equal([]byte("1"), b), nil
		},
		runtimeOS: []string{runtime.GOOS},
	},
	{
		/**
		*	echo "test" > testfile
		*	attr -l testfile
		*	attr -g "com.dropbox.ignored" testfile
		*	attr -s "com.dropbox.ignored" -V 1 testfile
		*	attr -l testfile
		*	attr -g "com.dropbox.ignored" testfile
		*	attr -r "com.dropbox.ignored" testfile
		*	attr -l testfile
		*	attr -g "com.dropbox.ignored" testfile
		*
		*	pi@raspberrypi:~ $ attr
		*	A filename to operate on is required
		*	Usage: attr [-LRSq] -s attrname [-V attrvalue] pathname  # set value
		*		   attr [-LRSq] -g attrname pathname                 # get value
		*		   attr [-LRSq] -r attrname pathname                 # remove attr
		*		   attr [-LRq]  -l pathname                          # list attrs
		*		   -s reads a value from stdin and -g writes a value to stdout
		*	pi@raspberrypi:~ $
		*
		* runner@fv-az1429-457:~/work/dropbox_ignore_service/dropbox_ignore_service$ attr --help
		* attr: invalid option -- '-'
		* Unrecognized option: ?
		* Usage: attr [-LRSq] -s attrname [-V attrvalue] pathname  # set value
		*        attr [-LRSq] -g attrname pathname                 # get value
		*        attr [-LRSq] -r attrname pathname                 # remove attr
		*        attr [-LRq]  -l pathname                          # list attrs
		*       -s reads a value from stdin and -g writes a value to stdout
		 */
		SetFlag: func(path string) error {
			return execCommand("attr", "-s", "com.dropbox.ignored", "-V", "1", path)
		},
		RemoveFlag: func(path string) error {
			return execCommand("attr", "-r", "com.dropbox.ignored", path)
		},
		HasFlag: func(path string) (bool, error) {
			out, err := execCommandGetOutput("attr", "-l", path)
			if err != nil {
				return false, fmt.Errorf("error getting attributes for file %s: %s", err, path)
			}

			if bytes.Contains(out, []byte("com.dropbox.ignored")) {
				out, err := execCommandGetOutput("attr", "-g", "com.dropbox.ignored", path)
				if err != nil {
					return false, fmt.Errorf("error getting com.dropbox.ignored attribute for file %s: %s", err, path)
				}

				if bytes.Equal(out, []byte("1")) {
					return true, nil
				}
			}

			return false, nil
		},
		runtimeOS: []string{runtime.GOOS},
	},
	{
		/**
		 * runner@fv-az1429-457:~/work/dropbox_ignore_service/dropbox_ignore_service$ xattr --help
		 * usage: xattr [-slz] file [file ...]
		 *        xattr -p [-slz] attr_name file [file ...]
		 *        xattr -w [-sz] attr_name attr_value file [file ...]
		 *        xattr -d [-s] attr_name file [file ...]
		 *
		 * The first form lists the names of all xattrs on the given file(s).
		 * The second form (-p) prints the value of the xattr attr_name.
		 * The third form (-w) sets the value of the xattr attr_name to attr_value.
		 * The fourth form (-d) deletes the xattr attr_name.
		 *
		 * options:
		 *   -h: print this help
		 *   -s: act on symbolic links themselves rather than their targets
		 *   -l: print long format (attr_name: attr_value)
		 *   -z: compress or decompress (if compressed) attribute value in zip format
		 * runner@fv-az1429-457:~/work/dropbox_ignore_service/dropbox_ignore_service$
		 *
		 *	echo "test" > testfile
		 *	xattr -w "user.com.dropbox.ignored" 1 testfile
		 *	xattr -l testfile
		 *	xattr -p "user.com.dropbox.ignored" testfile
		 *	xattr -d "user.com.dropbox.ignored" testfile
		 */
		SetFlag: func(path string) error {
			return execCommand("xattr", "-s", "user.com.dropbox.ignored", "-V", "1", path)
		},
		RemoveFlag: func(path string) error {
			return execCommand("xattr", "-d", "user.com.dropbox.ignored", path)
		},
		HasFlag: func(path string) (bool, error) {
			out, err := execCommandGetOutput("attr", "-l", path)
			if err != nil {
				return false, fmt.Errorf("error getting attributes for file %s: %s", err, path)
			}

			if bytes.Contains(out, []byte("user.com.dropbox.ignored")) {
				out, err := execCommandGetOutput("xattr", "-p", "user.com.dropbox.ignored", path)
				if err != nil {
					return false, fmt.Errorf("error getting com.dropbox.ignored attribute for file %s: %s", err, path)
				}

				if bytes.Equal(out, []byte("1")) {
					return true, nil
				}
			}

			return false, nil
		},
		runtimeOS: []string{runtime.GOOS},
	},
}

func init() {
	implementations = slices.DeleteFunc(implementations, func(i Implementation) bool {
		// return true to delete
		return !slices.Contains(i.runtimeOS, runtime.GOOS)
	})
}
