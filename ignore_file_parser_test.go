package main_test

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	main "github.com/anton15x/dropbox_ignore_service"
	"github.com/stretchr/testify/require"
)

func requireNoError(t *testing.T, err error) {
	if err != nil {
		require.Nil(t, err, "errored: %s", err.Error())
	}
}

func requireWriteToFile(t *testing.T, f io.Writer, data []byte) {
	n, err := f.Write(data)
	requireNoError(t, err)
	require.Equal(t, len(data), n)
}

func requireMkdir(t *testing.T, path string) {
	requireNoError(t, os.Mkdir(path, os.ModePerm))
}

func requireCloseFile(t *testing.T, f *os.File) {
	err := f.Close()
	if err != nil && !errors.Is(err, os.ErrClosed) {
		requireNoError(t, err)
	}
}

const IgnoreFileNameForIsIgnored = ".gitignore"

func skipTestIfGitIsNotInstalled(t *testing.T) {
	_, err := exec.LookPath("git")
	if err != nil {
		t.Logf("git seems not to be installed: %s", err)
		t.SkipNow()
	}
}

type GitRepo struct {
	root string
}

func NewGitRepo(root string) (*GitRepo, error) {
	r := &GitRepo{
		root: root,
	}
	err := r.init()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (g *GitRepo) init() error {
	cmd := exec.Command("git", "init")
	cmd.Dir = g.root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error: %s, out: %s", err, string(out))
	}

	return nil
}

func (g *GitRepo) IsIgnored(path string) (bool, error) {
	// https://git-scm.com/docs/git-check-ignore
	cmd := exec.Command("git", "check-ignore", path)
	cmd.Dir = g.root
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		exitError, ok := err.(*exec.ExitError)
		if !ok {
			return false, err
		}
		exitCode = exitError.ExitCode()
	}
	if exitCode == 0 {
		// 0 = One or more of the provided paths is ignored
		return true, nil
	}
	if exitCode == 1 {
		// 1 = None of the provided paths are ignored.
		return false, nil
	}

	// 128 = A fatal error was encountered.
	return false, fmt.Errorf("error: %s, out: %s", err, string(out))
}

func TestParseIgnoreFileFromBytes(t *testing.T) {
	// go test -v -timeout 30s -run ^TestParseIgnoreFileFromBytes$ github.com/anton15x/dropbox_ignore_service
	rootDir := t.TempDir()

	type iTestFolder struct {
		path    string
		ignored bool
	}
	tests := []struct {
		name    string
		prepare func(t *testing.T, root string)
		folders []*iTestFolder
	}{
		{
			name: "blank_lines",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "\n\n")
			},
			folders: []*iTestFolder{
				{filepath.Join("node_modules"), false},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "keep"), false},
				{filepath.Join("sub", "node_modules"), false},
			},
		},
		{
			name: "comment",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "#node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("node_modules"), false},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "keep"), false},
				{filepath.Join("sub", "node_modules"), false},
			},
		},
		{
			name: "comment_escaped",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "\\#node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("#node_modules"), true},
				{filepath.Join("node_modules"), false},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "keep"), false},
				{filepath.Join("sub", "node_modules"), false},
			},
		},
		{
			name: "only_spaces",
			prepare: func(t *testing.T, root string) {
				// pre escapes do not get trimmed, only trailing ones
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "  ")
			},
			folders: []*iTestFolder{
				{filepath.Join(" "), false},
				{filepath.Join("  "), false},
			},
		},
		{
			name: "trailing_spaces",
			prepare: func(t *testing.T, root string) {
				// pre escapes do not get trimmed, only trailing ones
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "  pre_spaces\npost_spaces  ")
			},
			folders: []*iTestFolder{
				{filepath.Join("  pre_spaces"), true},
				{filepath.Join(" pre_spaces"), false},
				{filepath.Join("pre_spaces"), false},
				{filepath.Join("post_spaces"), true},
				{filepath.Join("post_spaces "), false},
				{filepath.Join("post_spaces  "), false},
				{filepath.Join("post_spaces   "), false},
			},
		},
		{
			name: "trailing_spaces_escaped",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "\\  pre_spaces\npost_spaces \\ ")
			},
			folders: []*iTestFolder{
				{filepath.Join("  pre_spaces"), true},
				{filepath.Join(" pre_spaces"), false},
				{filepath.Join("pre_spaces"), false},
				{filepath.Join("post_spaces"), false},
				{filepath.Join("post_spaces "), false},
				{filepath.Join("post_spaces  "), true},
				{filepath.Join("post_spaces   "), false},
			},
		},
		{
			name: "negation_escaped",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "\\!important!.txt")
			},
			folders: []*iTestFolder{
				{filepath.Join("!important!.txt"), true},
				{filepath.Join("important!.txt"), false},
			},
		},
		{
			name: "negation_with_space_before_is_no_negation",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), " !important!.txt")
			},
			folders: []*iTestFolder{
				{filepath.Join(" !important!.txt"), true},
				{filepath.Join(" important!.txt"), false},
			},
		},
		{
			name: "base_name",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("node_modules"), true},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "keep"), false},
				{filepath.Join("sub", "node_modules"), true},
				{filepath.Join("sub", "node_modules", "node_modules"), true},
			},
		},
		{
			name: "base_name_and_subfolder",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "my_project/node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("node_modules"), false},
				{filepath.Join("my_project"), false},
				{filepath.Join("my_project", "src"), false},
				{filepath.Join("my_project", "node_modules"), true},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "node_modules"), false},
				{filepath.Join("sub", "my_project"), false},
				{filepath.Join("sub", "my_project", "node_modules"), false},
				{filepath.Join("sub", "my_project", "node_modules", "my_project"), false},
				{filepath.Join("sub", "my_project", "node_modules", "my_project", "node_modules"), false},
			},
		},
		{
			name: "pattern_root_folder",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "/node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("node_modules"), true},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "keep"), false},
				{filepath.Join("sub", "node_modules"), false},
			},
		},
		{
			name: "pattern_root_with_subfolder",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "/my_project/node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("my_project"), false},
				{filepath.Join("my_project", "src"), false},
				{filepath.Join("my_project", "node_modules"), true},
				{filepath.Join("subfolder"), false},
				{filepath.Join("subfolder", "my_project"), false},
				{filepath.Join("subfolder", "my_project", "node_modules"), false},
				{filepath.Join("subfolder", "node_modules"), false},
			},
		},
		{
			name: "ignore_file_in_subfolder",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "a")
				createDropboxignore(t, filepath.Join(root, "sub", IgnoreFileNameForIsIgnored), "b")
			},
			folders: []*iTestFolder{
				{filepath.Join("a"), true},
				{filepath.Join("b"), false},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "a"), true},
				{filepath.Join("sub", "b"), true},
				{filepath.Join("sub", "sub2"), false},
				{filepath.Join("sub", "sub2", "a"), true},
				{filepath.Join("sub", "sub2", "b"), true},
				{filepath.Join("sub2"), false},
				{filepath.Join("sub2", "a"), true},
				{filepath.Join("sub2", "b"), false},
			},
		},
		{
			name: "ignore_file_in_subfolder_slash_prefixed",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "/a")
				createDropboxignore(t, filepath.Join(root, "sub", IgnoreFileNameForIsIgnored), "/b")
			},
			folders: []*iTestFolder{
				{filepath.Join("a"), true},
				{filepath.Join("b"), false},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "a"), false},
				{filepath.Join("sub", "b"), true},
				{filepath.Join("sub", "sub2"), false},
				{filepath.Join("sub", "sub2", "a"), false},
				{filepath.Join("sub", "sub2", "b"), false},
				{filepath.Join("sub2"), false},
				{filepath.Join("sub2", "a"), false},
				{filepath.Join("sub2", "b"), false},
			},
		},
		{
			name: "asterisk",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "*.exe\nn*.log")
			},
			folders: []*iTestFolder{
				{filepath.Join(".exe"), true},
				{filepath.Join("go.exe"), true},
				{filepath.Join("gofmt.exe"), true},
				{filepath.Join("n.log"), true},
				{filepath.Join("nothing.log"), true},
				{filepath.Join("something.log"), false},
				{filepath.Join("bin"), false},
				{filepath.Join("bin", "test.exe"), true},
				{filepath.Join("bin", ".exe"), true},
				{filepath.Join("n"), false},
				{filepath.Join("n", ".log"), false}, // path separator not matched`by *
				{filepath.Join("n", "n.log"), true},
				{filepath.Join("n", "nothing.log"), true},
				{filepath.Join("n", "something.log"), false},
			},
		},
		{
			name: "asterisk_double_leading",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "**/foo/bar")
			},
			folders: []*iTestFolder{
				{filepath.Join("foo"), false},
				{filepath.Join("foo", "bar"), true},
				{filepath.Join("bar"), false},
				{filepath.Join("bar", "foo"), false},
				{filepath.Join("bar", "foo", "bar"), true},
			},
		},
		{
			name: "asterisk_double_trailing",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "abc/**")
			},
			folders: []*iTestFolder{
				{filepath.Join("abc"), false},
				{filepath.Join("abc", "foo"), true},
				{filepath.Join("abc", "bar"), true},
			},
		},
		{
			name: "asterisk_double_middle_at_slashes",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "a/**/b")
			},
			folders: []*iTestFolder{
				{filepath.Join("a"), false},
				{filepath.Join("a", "b"), true},
				{filepath.Join("a", "x"), false},
				{filepath.Join("a", "x", "b"), true},
				{filepath.Join("a", "x", "y"), false},
				{filepath.Join("a", "x", "y", "b"), true},
			},
		},
		{
			name: "asterisk_triple_leading",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "***/foo/bar")
			},
			folders: []*iTestFolder{
				{filepath.Join("foo"), false},
				{filepath.Join("foo", "bar"), true},
				{filepath.Join("bar"), false},
				{filepath.Join("bar", "foo"), false},
				{filepath.Join("bar", "foo", "bar"), true},
			},
		},
		{
			name: "asterisk_triple_trailing",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "abc/***")
			},
			folders: []*iTestFolder{
				{filepath.Join("abc"), false},
				{filepath.Join("abc", "foo"), true},
				{filepath.Join("abc", "bar"), true},
			},
		},
		{
			name: "asterisk_triple_middle_at_slashes",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "a/***/b")
			},
			folders: []*iTestFolder{
				{filepath.Join("a"), false},
				{filepath.Join("a", "b"), true},
				{filepath.Join("a", "x"), false},
				{filepath.Join("a", "x", "b"), true},
				{filepath.Join("a", "x", "y"), false},
				{filepath.Join("a", "x", "y", "b"), true},
			},
		},
		{
			name: "question_mark",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "a?a\n?b\nc?")
			},
			folders: []*iTestFolder{
				{filepath.Join("aza"), true},
				{filepath.Join("aa"), false},
				{filepath.Join("a"), false},
				{filepath.Join("a", "a"), false}, // no path separator match allowed
				{filepath.Join("zb"), true},
				{filepath.Join("a", "b"), false}, // no path separator match allowed
				{filepath.Join("b"), false},
				{filepath.Join("cz"), true},
				{filepath.Join("c"), false},
				{filepath.Join("sub"), false},
				{filepath.Join("sub", "aza"), true},
				{filepath.Join("sub", "zb"), true},
				{filepath.Join("sub", "cz"), true},
			},
		},
		{
			name: "square_brackets_character",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.[ab]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.[ab]"), false},
				{filepath.Join("test.a"), true},
				{filepath.Join("test.b"), true},
				{filepath.Join("test.ab"), false},
				{filepath.Join("test.c"), false},
				{filepath.Join("test.d"), false},
				{filepath.Join("test.cd"), false},
			},
		},
		{
			name: "square_brackets_character_exclamation",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.[!ab]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), false},
				{filepath.Join("test.b"), false},
				{filepath.Join("test.ab"), false},
				{filepath.Join("test.c"), true},
				{filepath.Join("test.d"), true},
				{filepath.Join("test.cd"), false},
			},
		},
		{
			name: "square_brackets_character_exclamation_escaped",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.[\\!]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), false},
				{filepath.Join("test.!"), true},
			},
		},
		{
			name: "square_brackets_character_exclamation_in_middle",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.[a!]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), true},
				{filepath.Join("test.b"), false},
				{filepath.Join("test.c"), false},
				{filepath.Join("test.!"), true},
			},
		},
		{
			name: "square_brackets_character_range",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.[a-c]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), true},
				{filepath.Join("test.b"), true},
				{filepath.Join("test.ab"), false},
				{filepath.Join("test.c"), true},
				{filepath.Join("test.d"), false},
				{filepath.Join("test.cd"), false},
			},
		},
		{
			name: "square_brackets_character_range_multiple",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.[a-cx-z]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), true},
				{filepath.Join("test.b"), true},
				{filepath.Join("test.ab"), false},
				{filepath.Join("test.c"), true},
				{filepath.Join("test.d"), false},
				{filepath.Join("test.cd"), false},
				{filepath.Join("test.w"), false},
				{filepath.Join("test.x"), true},
				{filepath.Join("test.y"), true},
				{filepath.Join("test.z"), true},
			},
		},
		{
			name: "square_brackets_character_range_exclamation",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.[!a-c]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), false},
				{filepath.Join("test.b"), false},
				{filepath.Join("test.ab"), false},
				{filepath.Join("test.c"), false},
				{filepath.Join("test.d"), true},
				{filepath.Join("test.cd"), false},
			},
		},
		{
			name: "square_brackets_character_range_multiple_exclamation",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.[!a-cx-z]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), false},
				{filepath.Join("test.b"), false},
				{filepath.Join("test.ab"), false},
				{filepath.Join("test.c"), false},
				{filepath.Join("test.d"), true},
				{filepath.Join("test.cd"), false},
				{filepath.Join("test.w"), true},
				{filepath.Join("test.x"), false},
				{filepath.Join("test.y"), false},
				{filepath.Join("test.z"), false},
			},
		},
		{
			name: "square_brackets_escaped",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.\\[ab\\]")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.[ab]"), true},
				{filepath.Join("test.a"), false},
				{filepath.Join("test.b"), false},
				{filepath.Join("test.ab"), false},
			},
		},
		{
			name: "curly_brackets_git_compatibility",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "test.{log,exe}")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), false},
				{filepath.Join("test.exe"), false}, // true if supported
				{filepath.Join("test.log"), false}, // true if supported
				{filepath.Join("test.{log,exe}"), true},
				{filepath.Join("test.txt"), false},
			},
		},
		{
			name: "/",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, IgnoreFileNameForIsIgnored), "/")
			},
			folders: []*iTestFolder{
				{filepath.Join("test.a"), false},
			},
		},
	}
	for testI, test := range tests {
		for testVariant := 0; testVariant <= 1; testVariant++ {
			testName := test.name
			compareGit := testVariant%2 == 0
			if compareGit {
				testName += "_compare_git"
			} else {
				testName += "_compare_hardcoded"
			}
			t.Run(testName, func(t *testing.T) {
				if compareGit {
					skipTestIfGitIsNotInstalled(t)
				}

				testRootDir := filepath.Join(rootDir, strconv.Itoa(testI)+"_"+strconv.Itoa(testVariant))
				err := os.Mkdir(testRootDir, os.ModePerm)
				requireNoError(t, err)

				test := test
				folders := make([]*iTestFolder, len(test.folders))
				for i, folder := range test.folders {
					f := *folder
					f.path = filepath.Join(testRootDir, folder.path)

					folders[i] = &f
				}
				test.folders = folders

				test.prepare(t, testRootDir)

				parsed, err := main.ParseIgnoreFilesFromRoot(testRootDir, IgnoreFileNameForIsIgnored)
				requireNoError(t, err)

				defer func() {
					log.Printf("defer of test %s", test.name)
					if t.Failed() {
						t.Logf("patterns for test %s:", test.name)
						for i, p := range parsed {
							t.Logf("p[%d]: %q", i, p)
						}
					}
				}()

				rmAllFolders := false
				for _, folder := range test.folders {
					folderPath := folder.path
					if strings.HasSuffix(folderPath, " ") {
						if runtime.GOOS == "windows" {
							require.True(t, filepath.IsAbs(folderPath))
							folderPath = "\\\\?\\" + folderPath
							rmAllFolders = true
						}
						t.Logf("warning: spaces in filename, test case could error")
					}

					err = os.Mkdir(folderPath, os.ModePerm)
					if err == nil || !os.IsExist(err) {
						requireNoError(t, err)
					}
					if rmAllFolders {
						// os.RemoveAll is unable to remove folders with backslash
						// => we remove all folders after the first created space folders ourself
						defer func() {
							err = os.Remove(folderPath)
							requireNoError(t, err)
						}()
					}
				}

				initGitOnce := sync.OnceValue(func() *GitRepo {
					g, err := NewGitRepo(testRootDir)
					requireNoError(t, err)
					return g
				})
				for _, folder := range test.folders {
					isIgnored := main.IsIgnored(parsed, folder.path)
					if compareGit {
						g := initGitOnce()
						expectedIsIgnored, err := g.IsIgnored(folder.path)
						requireNoError(t, err)
						require.Equal(t, expectedIsIgnored, isIgnored, "git mismatch: %q", folder.path)
					} else {
						require.Equal(t, folder.ignored, isIgnored, "expected mismatch: %q", folder.path)
					}
				}

			})
		}
	}
}
