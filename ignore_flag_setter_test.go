package main_test

import (
	"os"
	"path/filepath"
	"testing"

	main "github.com/anton15x/dropbox_ignore_service"
	"github.com/stretchr/testify/require"
)

func checkIsIgnored(t *testing.T, path string, expectedIsIgnored bool, msg string) {
	isIgnored, err := main.HasDropboxIgnoreFlag(path)
	require.Nil(t, err, msg)
	require.Equal(t, expectedIsIgnored, isIgnored, msg)
}

func TestIgnoreFlagModify(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		filename func(t *testing.T) string
	}{
		{
			name: "testFolder",
			filename: func(t *testing.T) string {
				n := filepath.Join(dir, "testFolder")
				require.Nil(t, os.Mkdir(n, os.ModePerm))
				return n
			},
		},
		{
			name: "testFile",
			filename: func(t *testing.T) string {
				n := filepath.Join(dir, "testFile")
				require.Nil(t, os.WriteFile(n, []byte{}, os.ModePerm))
				return n
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			filename := test.filename(t)
			checkIsIgnored(t, filename, false, "tracked initial")

			err := main.SetDropboxIgnoreFlag(filename)
			if err != nil {
				require.Nil(t, err, err.Error())
			}
			checkIsIgnored(t, filename, true, "ignored after adding ignore flag")

			err = main.SetDropboxIgnoreFlag(filename)
			if err != nil {
				require.Nil(t, err, err.Error())
			}
			checkIsIgnored(t, filename, true, "ignored after adding ignore flag twice")

			require.Nil(t, main.RemoveDropboxIgnoreFlag(filename))
			checkIsIgnored(t, filename, false, "tracked after removing ignore flag")

			require.Nil(t, main.RemoveDropboxIgnoreFlag(filename))
			checkIsIgnored(t, filename, false, "tracked after removing ignore flag twice")

			require.Nil(t, os.Remove(filename))
			// isIgnored, err = main.IsPathIgnored(filename)
			// require.NotNil(t, err)
			// require.True(t, errors.Is(err, os.ErrNotExist))
		})
	}
}
