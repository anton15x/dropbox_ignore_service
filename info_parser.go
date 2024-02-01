package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

type DropboxInfoEntry struct {
	Path             string `json:"path"`
	Host             uint64 `json:"host"`
	IsTeam           bool   `json:"is_team"`
	SubscriptionType string `json:"subscription_type"`
}

// may be be personal os business, but make a map for simplicity
type DropboxInfo map[string]DropboxInfoEntry

func ParseDropboxInfoPaths() ([]string, error) {
	// possible config locations:
	// %APPDATA%\Dropbox\info.json
	// %LOCALAPPDATA%\Dropbox\info.json
	// ~/.dropbox/info.json

	locations := []string{}
	user, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("error getting user: %w", err)
	}
	locations = append(locations, filepath.Join(user.HomeDir, ".dropbox", "info.json"))

	appData, found := os.LookupEnv("APPDATA")
	if found {
		locations = append(locations, filepath.Join(appData, "Dropbox", "info.json"))
	}
	localAppData, found := os.LookupEnv("LOCALAPPDATA")
	if found {
		locations = append(locations, filepath.Join(localAppData, "Dropbox", "info.json"))
	}

	for _, location := range locations {
		bytes, err := os.ReadFile(location)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("error reading dropbox config file %s: %s", location, err)
			}
			continue
		}

		info := DropboxInfo{}
		err = json.Unmarshal(bytes, &info)
		if err != nil {
			return nil, fmt.Errorf("error parsing dropbox config file %s: %s", location, err)
		}

		paths := []string{}
		for _, value := range info {
			paths = append(paths, value.Path)
		}
		return paths, nil
	}

	// no dropbox info file found
	// return nil, nil
	return nil, fmt.Errorf("no dropbox info file found")
}
