package filewalkfast_test

import (
	"cmp"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/anton15x/dropbox_ignore_service/src/filewalkfast"
	"github.com/anton15x/dropbox_ignore_service/src/util"
	"github.com/stretchr/testify/require"
)

func OrderSize(a, b fs.FileInfo) int {
	var sizeA int64
	var sizeB int64
	if !a.IsDir() {
		sizeA = a.Size()
	}
	if !b.IsDir() {
		sizeB = b.Size()
	}
	if r := cmp.Compare(sizeA, sizeB); r != 0 {
		return r
	}

	return cmp.Compare(a.Name(), b.Name())
}

type DataEntry struct {
	Path     string
	Size     int64
	SubEntry []*DataEntry
}

func infoToDataEntry(path string, info fs.FileInfo) *DataEntry {
	e := &DataEntry{Path: path}
	if !info.IsDir() {
		// windows return 0
		// linux and darwin 4096
		// => directories get set to for consistent behavior
		e.Size = info.Size()
	}
	return e
}

func OrderSizeDataEntry(a, b *DataEntry) int {
	if a := cmp.Compare(a.Size, b.Size); a != 0 {
		return a
	}

	return cmp.Compare(a.Path, b.Path)
}

func OrderByPathDataEntry(a, b *DataEntry) int {
	return cmp.Compare(a.Path, b.Path)
}

func BenchmarkFileWalkFast(b *testing.B) {
	type DataEntry struct {
		Path     string
		Size     int64
		SubEntry []*DataEntry
	}

	type implementation struct {
		name string
		f    func(root string, cb func(path string, err error) error) error
	}
	implementations := []implementation{
		{
			name: "filepath.Walk",
			f: func(root string, cb func(path string, err error) error) error {
				return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				})
			},
		},
		{
			name: "filepath.WalkDir",
			f: func(root string, cb func(path string, err error) error) error {
				return filepath.WalkDir(root, func(path string, info fs.DirEntry, err error) error {
					return cb(path, err)
				})
			},
		},
		{
			name: "Walk",
			f: func(root string, cb func(path string, err error) error) error {
				return filewalkfast.Walk(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				})
			},
		},
		{
			name: "WalkOrder-unsorted",
			f: func(root string, cb func(path string, err error) error) error {
				return filewalkfast.WalkOrder(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				}, nil)
			},
		},
		{
			name: "WalkOrder-size",
			f: func(root string, cb func(path string, err error) error) error {
				return filewalkfast.WalkOrder(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				}, OrderSize)
			},
		},
		{
			name: "WalkUnorderedMemoryAware",
			f: func(root string, cb func(path string, err error) error) error {
				return filewalkfast.WalkUnorderedMemoryAware(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				})
			},
		},
		{
			name: "WalkData-*DataEntry",
			f: func(root string, cb func(path string, err error) error) error {
				_, err := filewalkfast.WalkData(root, func(path string, info fs.FileInfo, err error, data *DataEntry) (*DataEntry, error) {
					e := &DataEntry{Path: path, Size: info.Size()}
					data.SubEntry = append(data.SubEntry, e)
					return e, cb(path, err)
				}, &DataEntry{})
				return err
			},
		},
		{
			name: "WalkDataOrder-unsorted-*DataEntry",
			f: func(root string, cb func(path string, err error) error) error {
				_, err := filewalkfast.WalkDataOrder(root, func(path string, info fs.FileInfo, err error, data *DataEntry) (*DataEntry, error) {
					e := &DataEntry{Path: path, Size: info.Size()}
					data.SubEntry = append(data.SubEntry, e)
					return e, cb(path, err)
				}, &DataEntry{}, nil)
				return err
			},
		},
		{
			name: "WalkDataOrder-OrderSize-*DataEntry",
			f: func(root string, cb func(path string, err error) error) error {
				_, err := filewalkfast.WalkDataOrder(root, func(path string, info fs.FileInfo, err error, data *DataEntry) (*DataEntry, error) {
					e := &DataEntry{Path: path, Size: info.Size()}
					data.SubEntry = append(data.SubEntry, e)
					return e, cb(path, err)
				}, &DataEntry{}, OrderSize)
				return err
			},
		},
		{
			name: "WalkDataUnorderedMemoryAware-*DataEntry",
			f: func(root string, cb func(path string, err error) error) error {
				_, err := filewalkfast.WalkDataUnorderedMemoryEfficient(root, func(path string, info fs.FileInfo, err error, data *DataEntry) (*DataEntry, error) {
					e := &DataEntry{Path: path, Size: info.Size()}
					data.SubEntry = append(data.SubEntry, e)
					return e, cb(path, err)
				}, &DataEntry{})
				return err
			},
		},
	}
	util.PadStringsF("_", implementations, func(t *implementation) string { return t.name }, func(val *implementation, newValue string) { val.name = newValue })

	tests := []struct {
		name string
		dir  string
	}{
		{
			// `\\?\` is disables all path normalizations
			// with that, files/folders with trailing spaces are supported
			name: "tmp",
			dir:  `\\?\` + os.TempDir(),
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			for _, impl := range implementations {
				b.Run(impl.name, func(b *testing.B) {
					var err error
					var files int64
					for range b.N {
						err = impl.f(test.dir, func(path string, err error) error {
							files++
							return err
						})
					}
					b.StopTimer()
					require.NoError(b, err)
					b.ReportMetric(float64(files/int64(b.N)), "files/op")
				})
			}
		})
	}
}

func TestFileWalkFast(t *testing.T) {
	implementations := []struct {
		name     string
		f        func(root string, cb func(path string, err error) error) (*DataEntry, error)
		isSorted bool
		sortF    func(a, b *DataEntry) int
	}{
		{
			name: "filepath.Walk",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				return nil, filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				})
			},
			isSorted: true,
			sortF:    OrderByPathDataEntry,
		},
		{
			name: "filepath.WalkDir",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				return nil, filepath.WalkDir(root, func(path string, info fs.DirEntry, err error) error {
					return cb(path, err)
				})
			},
			isSorted: true,
			sortF:    OrderByPathDataEntry,
		},
		{
			name: "Walk",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				return nil, filewalkfast.Walk(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				})
			},
			isSorted: true,
			sortF:    OrderByPathDataEntry,
		},
		{
			name: "WalkOrder-unsorted",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				return nil, filewalkfast.WalkOrder(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				}, nil)
			},
		},
		{
			name: "WalkOrder-size",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				return nil, filewalkfast.WalkOrder(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				}, OrderSize)
			},
			isSorted: true,
			sortF:    OrderSizeDataEntry,
		},
		{
			name: "WalkUnorderedMemoryAware",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				return nil, filewalkfast.WalkUnorderedMemoryAware(root, func(path string, info fs.FileInfo, err error) error {
					return cb(path, err)
				})
			},
		},
		{
			name: "WalkData-*DataEntry",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				e, err := filewalkfast.WalkData(root, func(path string, info fs.FileInfo, err error, data *DataEntry) (*DataEntry, error) {
					e := infoToDataEntry(path, info)
					data.SubEntry = append(data.SubEntry, e)
					return e, cb(path, err)
				}, &DataEntry{})
				return e, err
			},
			isSorted: true,
			sortF:    OrderByPathDataEntry,
		},
		{
			name: "WalkDataOrder-unsorted-*DataEntry",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				e, err := filewalkfast.WalkDataOrder(root, func(path string, info fs.FileInfo, err error, data *DataEntry) (*DataEntry, error) {
					e := infoToDataEntry(path, info)
					data.SubEntry = append(data.SubEntry, e)
					return e, cb(path, err)
				}, &DataEntry{}, nil)
				return e, err
			},
		},
		{
			name: "WalkDataOrder-OrderSize-*DataEntry",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				e, err := filewalkfast.WalkDataOrder(root, func(path string, info fs.FileInfo, err error, data *DataEntry) (*DataEntry, error) {
					e := infoToDataEntry(path, info)
					data.SubEntry = append(data.SubEntry, e)
					return e, cb(path, err)
				}, &DataEntry{}, OrderSize)
				return e, err
			},
			isSorted: true,
			sortF:    OrderSizeDataEntry,
		},
		{
			name: "WalkDataUnorderedMemoryAware-*DataEntry",
			f: func(root string, cb func(path string, err error) error) (*DataEntry, error) {
				e, err := filewalkfast.WalkDataUnorderedMemoryEfficient(root, func(path string, info fs.FileInfo, err error, data *DataEntry) (*DataEntry, error) {
					e := infoToDataEntry(path, info)
					data.SubEntry = append(data.SubEntry, e)
					return e, cb(path, err)
				}, &DataEntry{})
				return e, err
			},
		},
	}

	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "file_a.txt")
	err := os.WriteFile(fileA, []byte{}, os.ModePerm)
	require.NoError(t, err)

	fileB := filepath.Join(tmpDir, "file_b.txt")
	err = os.WriteFile(fileB, []byte{'1'}, os.ModePerm)
	require.NoError(t, err)

	fileC := filepath.Join(tmpDir, "file_c.txt")
	err = os.WriteFile(fileC, []byte{'1', '2'}, os.ModePerm)
	require.NoError(t, err)

	dirA := filepath.Join(tmpDir, "dir_a")
	err = os.Mkdir(dirA, os.ModePerm)
	require.NoError(t, err)

	fileA_A := filepath.Join(dirA, "file_a.txt")
	err = os.WriteFile(fileA_A, []byte{}, os.ModePerm)
	require.NoError(t, err)

	dirA_A := filepath.Join(dirA, "dir_a")
	err = os.Mkdir(dirA_A, os.ModePerm)
	require.NoError(t, err)

	fileA_A_A := filepath.Join(dirA_A, "file_a.txt")
	err = os.WriteFile(fileA_A_A, []byte{}, os.ModePerm)
	require.NoError(t, err)

	dirB := filepath.Join(tmpDir, "dir_b")
	err = os.Mkdir(dirB, os.ModePerm)
	require.NoError(t, err)

	fileB_A := filepath.Join(dirB, "file_a.txt")
	err = os.WriteFile(fileB_A, []byte{}, os.ModePerm)
	require.NoError(t, err)

	dirB_A := filepath.Join(dirB, "dir_a")
	err = os.Mkdir(dirB_A, os.ModePerm)
	require.NoError(t, err)

	fileB_A_A := filepath.Join(dirB_A, "file_a.txt")
	err = os.WriteFile(fileB_A_A, []byte{}, os.ModePerm)
	require.NoError(t, err)

	data := &DataEntry{
		Path: tmpDir,
		SubEntry: []*DataEntry{
			{
				Path: fileA,
			},
			{
				Path: fileB,
				Size: 1,
			},
			{
				Path: fileC,
				Size: 2,
			},
			{
				Path: dirA,
				SubEntry: []*DataEntry{
					{
						Path: fileA_A,
					},
					{
						Path: dirA_A,
						SubEntry: []*DataEntry{
							{
								Path: fileA_A_A,
							},
						},
					},
				},
			},
			{
				Path: dirB,
				SubEntry: []*DataEntry{
					{
						Path: fileB_A,
					},
					{
						Path: dirB_A,
						SubEntry: []*DataEntry{
							{
								Path: fileB_A_A,
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		dir  string
		data *DataEntry
	}{
		{
			name: "test",
			dir:  tmpDir,
			data: data,
		},
	}

	var sortData func(s *DataEntry, f func(a, b *DataEntry) int)
	sortData = func(s *DataEntry, f func(a, b *DataEntry) int) {
		slices.SortFunc(s.SubEntry, f)
		for _, s = range s.SubEntry {
			sortData(s, f)
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("dir %s", test.dir)

			for _, impl := range implementations {
				t.Run(impl.name, func(t *testing.T) {
					sortF := OrderByPathDataEntry
					if impl.sortF != nil {
						sortF = impl.sortF
					}
					sortData(test.data, sortF)

					files := []string{}
					var f func(e *DataEntry)
					f = func(e *DataEntry) {
						files = append(files, e.Path)
						for _, s := range e.SubEntry {
							f(s)
						}
					}
					f(test.data)

					var err error
					var fileI int
					checked := map[string]struct{}{}

					data, err := impl.f(test.dir, func(path string, err error) error {
						require.NoError(t, err)
						_, ok := checked[path]
						require.False(t, ok, "check file %s", path)
						checked[path] = struct{}{}
						require.True(t, slices.Contains(files, path), "path %s", path)

						if path != test.dir {
							// parent must be called first
							_, ok := checked[filepath.Dir(path)]
							require.True(t, ok, "check file %s", filepath.Dir(path))
						}

						if impl.isSorted {
							require.True(t, fileI < len(files))
							require.Equal(t, files[fileI], path)
						}
						fileI++
						return err
					})

					require.NoError(t, err)
					require.Equal(t, len(files), fileI)
					for _, file := range files {
						_, ok := checked[file]
						require.True(t, ok, "check file %s", file)
					}
					if data != nil {
						if !impl.isSorted {
							sortData(data, sortF)
						}
						require.Equal(t, test.data, data)
					}
				})
			}
		})
	}
}
