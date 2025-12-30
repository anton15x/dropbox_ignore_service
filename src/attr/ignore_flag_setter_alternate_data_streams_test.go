//go:build windows

package attr

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkHasFlag(b *testing.B) {
	impl := []struct {
		name string
		f    func(path string) (bool, error)
	}{
		{
			name: "implementation.HasFlag",
			f:    implementation.HasFlag,
		},
		{
			name: "HasFlagSimple",
			f:    HasFlagSimple,
		},
		{
			name: "HasFlagSyscall",
			f:    HasFlagSyscall,
		},
	}

	tmpDir := b.TempDir()

	hasFlagFile := filepath.Join(tmpDir, "____________________has_flag.txt")
	err := os.WriteFile(hasFlagFile, []byte{}, os.ModePerm)
	require.NoError(b, err)
	err = implementation.SetFlag(hasFlagFile)
	require.NoError(b, err)

	noFlag := filepath.Join(tmpDir, "no_flag.txt")
	err = os.WriteFile(hasFlagFile, []byte{}, os.ModePerm)
	require.NoError(b, err)

	falseFlagZero := filepath.Join(tmpDir, "false_flag_zero.txt")
	err = os.WriteFile(hasFlagFile, []byte{}, os.ModePerm)
	require.NoError(b, err)
	err = os.WriteFile(falseFlagZero+":com.dropbox.ignored", []byte{'0'}, os.ModePerm)
	require.NoError(b, err)

	falseFlagEmpty := filepath.Join(tmpDir, "false_flag_empty.txt")
	err = os.WriteFile(hasFlagFile, []byte{}, os.ModePerm)
	require.NoError(b, err)
	err = os.WriteFile(falseFlagEmpty+":com.dropbox.ignored", []byte{}, os.ModePerm)
	require.NoError(b, err)

	falseFlagLong := filepath.Join(tmpDir, "false_flag_long.txt")
	err = os.WriteFile(hasFlagFile, []byte{}, os.ModePerm)
	require.NoError(b, err)
	err = os.WriteFile(falseFlagLong+":com.dropbox.ignored", bytes.Repeat([]byte{'1'}, 100), os.ModePerm)
	require.NoError(b, err)

	notExitsFile := filepath.Join(tmpDir, "not_exists.txt")

	tests := []struct {
		filename string
		hasFlag  bool
		checkErr func(t testing.TB, err error)
	}{
		{
			filename: hasFlagFile,
			hasFlag:  true,
		},
		{
			filename: noFlag,
			hasFlag:  false,
		},
		{
			filename: falseFlagZero,
			hasFlag:  false,
		},
		{
			filename: falseFlagEmpty,
			hasFlag:  false,
		},
		{
			filename: falseFlagLong,
			hasFlag:  false,
		},
		{
			filename: notExitsFile,
			hasFlag:  false,
		},
	}
	for _, test := range tests {
		b.Run(test.filename, func(b *testing.B) {
			for _, impl := range impl {
				b.Run(impl.name, func(b *testing.B) {
					var hasFlag bool
					var err error
					for range b.N {
						hasFlag, err = impl.f(test.filename)
					}
					b.StopTimer()

					assert.Equal(b, test.hasFlag, hasFlag)
					if test.checkErr != nil {
						require.Error(b, err)
						test.checkErr(b, err)
						return
					}
					require.NoError(b, err)
				})
			}
		})
	}
}

func HasFlagSimple(path string) (bool, error) {
	b, err := os.ReadFile(path + ":com.dropbox.ignored")
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(b, []byte("1")), nil
}

func HasFlagSyscall(path string) (bool, error) {
	full := path + ":com.dropbox.ignored"

	ptr, err := syscall.UTF16PtrFromString(full)
	if err != nil {
		return false, fmt.Errorf("UTF16PtrFromString %s: %w", full, err)
	}

	const (
		desiredAccess uint32 = syscall.GENERIC_READ
		shareMode     uint32 = syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE
		creationDisp  uint32 = syscall.OPEN_EXISTING
		flagsAttrs    uint32 = syscall.FILE_ATTRIBUTE_NORMAL
	)

	// Open ADS directly
	handle, err := syscall.CreateFile(
		ptr,
		desiredAccess,
		shareMode,
		nil,
		creationDisp,
		flagsAttrs,
		0,
	)
	if handle == 0 || handle == syscall.InvalidHandle {
		if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(err, syscall.ERROR_PATH_NOT_FOUND) {
			return false, nil
		}
		// ADS does not exist
		return false, fmt.Errorf("invalid handle (h=%d): %w", handle, err)
	}
	defer syscall.CloseHandle(handle)

	// Read exactly 1 byte
	var buf [2]byte
	var read uint32

	err = syscall.ReadFile(handle, buf[:], &read, nil)
	if err != nil {
		return false, fmt.Errorf("read ds stream %s: %w", full, err)
	}

	// Check content
	// Case 1: 1 byte read -> byte must equal 1 -> valid
	// Case 2: 0 bytes -> empty ADS -> invalid
	// Case 3: 2 bytes -> too long -> invalid
	return read == 1 && buf[0] == '1', nil
}
