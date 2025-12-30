package filewalkfast

// 5x as fast as filepath.Walk
// same speed as filepath.WalkDir
// this package is a replacement for filepath.Walk if fs.FileInfo is required or parent data needs to get passed to child nodes (WalkData)
// filepath.WalkDir is already optimized and performance same as filewalkfast

import (
	"cmp"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

func Walk(root string, fn filepath.WalkFunc) error {
	return WalkOrder(root, fn, func(a, b os.FileInfo) int {
		return cmp.Compare(a.Name(), b.Name())
	})
}

func WalkOrder(root string, fn filepath.WalkFunc, order func(a, b fs.FileInfo) int) error {
	_, err := WalkDataOrder(root, func(path string, info fs.FileInfo, err error, data struct{}) (struct{}, error) {
		return data, fn(path, info, err)
	}, struct{}{}, order)
	return err
}

func WalkUnorderedMemoryAware(root string, fn filepath.WalkFunc) error {
	_, err := WalkDataUnorderedMemoryEfficient(root, func(path string, info fs.FileInfo, err error, data struct{}) (struct{}, error) {
		return struct{}{}, fn(path, info, err)
	}, struct{}{})
	return err
}

type WalkDataFunc[T any] func(path string, info fs.FileInfo, err error, data T) (T, error)

func WalkData[T any](root string, fn WalkDataFunc[T], data T) (T, error) {
	return WalkDataOrder(root, fn, data, func(a, b fs.FileInfo) int {
		return cmp.Compare(a.Name(), b.Name())
	})
}

func WalkDataOrder[T any](root string, fn WalkDataFunc[T], data T, order func(a, b fs.FileInfo) int) (T, error) {
	info, err := os.Lstat(root)
	if err != nil {
		data, err = fn(root, nil, err, data)
	} else {
		data, err = walkData(root, info, fn, data, order)
	}
	if err == filepath.SkipDir || err == filepath.SkipAll {
		return data, nil
	}
	return data, err
}

func walkData[T any](path string, d fs.FileInfo, walkFn WalkDataFunc[T], data T, order func(a, b fs.FileInfo) int) (T, error) {
	newData, err := walkFn(path, d, nil, data)
	if err != nil || !d.IsDir() {
		if err == filepath.SkipDir && d.IsDir() {
			// Successfully skipped directory.
			err = nil
		}
		return newData, err
	}

	var infos []os.FileInfo
	func() {
		var f *os.File
		f, err = os.Open(path)
		if err == nil {
			//nolint:errcheck
			defer f.Close()

			// Readdir returns os.FileInfo directly, and is much faster, than stating each file separately
			// throwback: memory spikes possible
			infos, err = f.Readdir(-1)
		}
	}()

	if err != nil {
		// Second call, to report ReadDir error.
		_, err = walkFn(path, d, err, data)
		if err != nil {
			if err == filepath.SkipDir && d.IsDir() {
				err = nil
			}
			return newData, err
		}
	}

	if order != nil {
		slices.SortFunc(infos, order)
	}

	for _, info := range infos {
		path1 := filepath.Join(path, info.Name())
		if _, err := walkData(path1, info, walkFn, newData, order); err != nil {
			if err == filepath.SkipDir {
				break
			}
			return newData, err
		}
	}

	return newData, nil
}

func WalkDataUnorderedMemoryEfficient[T any](root string, fn WalkDataFunc[T], data T) (T, error) {
	info, err := os.Lstat(root)
	if err != nil {
		data, err = fn(root, nil, err, data)
	} else {
		data, err = walkDataUnorderedMemoryAware(root, info, fn, data)
	}
	if err == filepath.SkipDir || err == filepath.SkipAll {
		return data, nil
	}
	return data, err
}

func walkDataUnorderedMemoryAware[T any](path string, d fs.FileInfo, walkFn WalkDataFunc[T], data T) (T, error) {
	newData, err := walkFn(path, d, nil, data)
	if err != nil || !d.IsDir() {
		if err == filepath.SkipDir && d.IsDir() {
			// Successfully skipped directory.
			err = nil
		}
		return newData, err
	}

	f, err := os.Open(path)
	if err == nil {
		//nolint:errcheck
		defer f.Close()

		for {
			// Readdir returns os.FileInfo directly, and is much faster, than stating each file separately
			// limiting return values:
			// good: memory spikes less possible
			// readdir caches the handle so multiple calls to it are very performant
			// throwback: ordering not possible
			infos, err := f.Readdir(100)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return newData, nil
				}
				break
			}
			for _, info := range infos {
				path1 := filepath.Join(path, info.Name())
				if _, err := walkDataUnorderedMemoryAware(path1, info, walkFn, newData); err != nil {
					if err == filepath.SkipDir {
						break
					}
					return newData, err
				}
			}
		}
	}

	// Second call, to report ReadDir error.
	_, err = walkFn(path, d, err, data)
	if err != nil {
		if err == filepath.SkipDir && d.IsDir() {
			err = nil
		}
		return newData, err
	}
	return newData, nil
}
