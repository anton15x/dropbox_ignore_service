package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type IgnorePattern []string

func ParseIgnoreFilesFromRoot(dir string, ignoreFileName string) (IgnorePattern, error) {
	var patterns IgnorePattern
	err := filepath.WalkDir(dir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && IsIgnored(patterns, path) {
			return filepath.SkipDir
		}

		if filepath.Base(path) == ignoreFileName {
			p, err := ParseIgnoreFile(path)
			if err != nil {
				return fmt.Errorf("error parsing ignore file %s: %w", path, err)
			}
			patterns = append(patterns, p...)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking files in %s: %w", dir, err)
	}

	return patterns, nil
}

func ParseIgnoreFile(filename string) (IgnorePattern, error) {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading ignore file %s: %w", filename, err)
	}
	return ParseIgnoreFileFromBytes(filename, fileBytes)
}

// ignore rules:
// https://git-scm.com/docs/gitignore
func ParseIgnoreFileFromBytes(filename string, fileBytes []byte) (IgnorePattern, error) {
	var patterns IgnorePattern

	fileDir := filepath.Dir(filename)

	// ignoreLines := strings.Split(string(fileBytes), "\n")
	ignoreLines := regexp.MustCompile("\r?\n").Split(string(fileBytes), -1)
	for lineI, ignoreLine := range ignoreLines {
		// ignore comment line
		if strings.HasPrefix(ignoreLine, "#") {
			continue
		}

		if strings.HasPrefix(ignoreLine, "!") {
			return nil, fmt.Errorf("negation not allowed: %s", ignoreLine)
		}

		globPattern := ""

		parsedTilNow := ""
		ignoreLineRunes := []rune(ignoreLine)
		for i := 0; i < len(ignoreLineRunes); i++ {
			c := ignoreLineRunes[i]
			switch c {
			case '\\':
				if i+1 < len(ignoreLineRunes) {
					i++
					next := ignoreLineRunes[i]
					parsedTilNow += string(c)
					parsedTilNow += string(next)
					globPattern += parsedTilNow
					parsedTilNow = ""
				}
			case '*':
				// git thread multiple asterisks same as two
				// doublestar more than two as one
				// => keep single asterisk and make two or more asterisks to a double asterisk
				addC := 1
				for ; i+1 < len(ignoreLineRunes); i++ {
					if ignoreLineRunes[i+1] != '*' {
						break
					}
					addC = 2
				}
				parsedTilNow += strings.Repeat("*", addC)

				// git behavior: multiple star at end do not match last path itself
				// doublestar: double start also matches last path itself
				// => add /* to and to force matching any path
				if addC == 2 && i+1 >= len(ignoreLineRunes) {
					parsedTilNow += "/*"
				}
			case '{':
				fallthrough
			case '}':
				parsedTilNow += `\`
				parsedTilNow += string(c)
			default:
				parsedTilNow += string(c)
			}
		}
		parsedTilNow = strings.TrimRight(parsedTilNow, " ")
		globPattern += parsedTilNow

		if globPattern == "" {
			// empty line ore line with only spaces
			continue
		}

		// if a slash is in the path, independent where, it is always relative to ignore file
		if !strings.Contains(ignoreLine, "/") {
			globPattern = path.Join("**", globPattern)
		}
		globPattern = path.Join(filepath.ToSlash(fileDir), globPattern)

		valid := doublestar.ValidatePattern(globPattern)
		if !valid {
			return nil, fmt.Errorf("invalid pattern at line %d: %s created by line: %s", lineI, globPattern, ignoreLine)
		}
		patterns = append(patterns, globPattern)
	}

	return patterns, nil
}

func IsIgnored(patterns IgnorePattern, path string) bool {
	for _, ignorePattern := range patterns {
		match, err := doublestar.Match(ignorePattern, filepath.ToSlash(path))
		if err != nil {
			// bad
			panic(err)
		}
		if match {
			return true
		}
	}

	return false
}
