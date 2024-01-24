package main

import (
	"github.com/spiretechnology/go-autostart/v2"
)

// on windows: symlink at %USERPROFILE%/AppData/Roaming/Microsoft/Windows/Start Menu/Programs/Startup

func getAutoStart() autostart.Autostart {
	return autostart.New(autostart.Options{
		Label:       "com.github.anton15x.dropbox_ignore_service",
		Vendor:      "com.github.anton15x.dropbox_ignore_service",
		Name:        "Dropbox Ignore Service",
		Description: "Dropbox Ignore Service support for .dropboxignore file",
		Mode:        autostart.ModeUser,
		Arguments:   []string{},
	})
}

func EnableAutoStart() error {
	app := getAutoStart()

	err := app.Enable()
	if err != nil {
		return err
	}

	// // To get other useful data
	// app.DataDir()
	// app.StdOutPath()
	// app.StdErrPath()

	// TODO: logging?

	return nil
}

func DisableAutoStart() error {
	return getAutoStart().Disable()
}

func IsAutoStartEnabled() (bool, error) {
	return getAutoStart().IsEnabled()
}

/*
const runKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`

func setAutoStart(key, value string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.SetStringValue(key, value)
}

func unsetAutoStart(key string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.DeleteValue(key)
}

func getAutoStart(key string) (*string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()

	value, _, err := k.GetStringValue(key)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil, nil
		}
		return nil, err
	}

	return &value, nil
}
*/
