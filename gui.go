package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"fyne.io/systray"
)

func appNameToUserDisplay(a fyne.App) string {
	// return strings.Title(strings.ReplaceAll(a.Metadata().Name, "_", ""))
	ret := ""
	origName := a.Metadata().Name
	for i, r := range origName {
		if r == '_' {
			continue
		}
		if i == 0 || origName[i-1] == '_' {
			ret += strings.ToUpper(string(r))
		} else {
			ret += string(r)
		}
	}
	if ext := filepath.Ext(ret); strings.EqualFold(ext, ".exe") {
		ret, _ = strings.CutSuffix(ret, ext)
	}
	return ret
}

func ShowGUI(ctx context.Context, dropboxIgnorers []*DropboxIgnorer, hideGUI bool, ignoredPathsSet *SortedStringSet, ignoreFilesSet *SortedStringSet, logStringSlice *logStringSliceStruct) error {
	guiCtx := ctx

	// FyneApp.toml has id and icon set => fyne build adds metadata for us
	// do not use go build, instead use:
	// fyne package --release
	a := app.New()
	// a := app.NewWithID("dropbox_ignore_service")
	// w := a.NewWindow(a.Metadata().Name)
	w := a.NewWindow(appNameToUserDisplay(a))
	w.Resize(fyne.NewSize(1200, 800))

	ignoredPathsSetList := widget.NewList(
		func() int {
			return ignoredPathsSet.Len()
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			// multithreading
			name := ignoredPathsSet.GetOrEmptyString(i)

			label := o.(*widget.Label)
			label.SetText(name)
		},
	)
	homeTopLabel := widget.NewLabel("")
	updateHomeTopLabel := func() {
		homeTopLabel.SetText(fmt.Sprintf("Ignoring %d elements", ignoredPathsSet.Len()))
	}
	ignoredPathsSet.AddChangeEventListener(Debounce(func() {
		updateHomeTopLabel()
		ignoredPathsSetList.Refresh()
	}, time.Second/60))
	updateHomeTopLabel()
	homeContent := container.NewBorder(
		homeTopLabel,
		nil, nil, nil,
		ignoredPathsSetList,
	)
	homeTab := container.NewTabItemWithIcon("Home", theme.SettingsIcon(), homeContent)

	showOnlyRemovableOrAllFiles := true
	ignoredFileNames := NewSortedStringSet()
	checkedFileNames := NewSortedStringSet()
	ignoredFileNamesValuesLastLenCall := []string{}
	ignoredFilesListContent := widget.NewList(
		func() int {
			// saving values makes it multithreading safe
			ignoredFileNamesValuesLastLenCall = []string{}
			for _, val := range ignoredFileNames.Values() {
				if !showOnlyRemovableOrAllFiles || !ignoredPathsSet.Has(val) {
					ignoredFileNamesValuesLastLenCall = append(ignoredFileNamesValuesLastLenCall, val)
				}
			}
			return len(ignoredFileNamesValuesLastLenCall)
		},
		func() fyne.CanvasObject {
			var check *widget.Check
			check = widget.NewCheck("", func(b bool) {
				name := check.Text
				if b {
					checkedFileNames.Add(name)
				} else {
					checkedFileNames.Remove(name)
				}
			})
			return check
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			values := ignoredFileNamesValuesLastLenCall
			name := ""
			if i < len(values) {
				name = values[i]
			}

			check := o.(*widget.Check)
			check.SetText(name)
			check.Checked = checkedFileNames.Has(name)

			if ignoredPathsSet.Has(name) {
				check.Disable()
			} else {
				check.Enable()
			}

		},
	)

	ignoredFilesListContentRefreshDebounced := Debounce(func() {
		ignoredFilesListContent.Refresh()
	}, time.Second/60)

	ignoredFilesProgressBar := widget.NewProgressBar()
	ignoredFilesProgressBar.Max = float64(len(dropboxIgnorers))
	ignoredFilesProgressCurrentDropboxPath := widget.NewLabel("")
	ignoredFilesProgressCurrentPath := widget.NewLabel("")
	ignoredFilesProgress := container.NewVBox(
		ignoredFilesProgressBar,
		container.New(layout.NewHBoxLayout(), widget.NewLabel("current dropbox root:"), ignoredFilesProgressCurrentDropboxPath),
		container.New(layout.NewHBoxLayout(), widget.NewLabel("current path:"), ignoredFilesProgressCurrentPath),
	)
	ignoredPathsSet.AddChangeEventListener(func() {
		ignoredFilesListContentRefreshDebounced()
	})
	ignoredPathsSet.AddAddEventListener(func(s string) {
		ignoredFileNames.Add(s)
		a.SendNotification(fyne.NewNotification("DropboxIgnoreFlag added", s))
	})
	ignoredPathsSet.AddRemoveEventListener(func(s string) {
		ignoredFileNames.Remove(s)
	})
	ignoredFileNames.AddChangeEventListener(func() {
		ignoredFilesListContentRefreshDebounced()
	})
	ignoredFileNames.AddRemoveEventListener(func(s string) {
		checkedFileNames.Remove(s)
	})
	ignoredFilesProgressCurrentPathRefreshDebounced := Debounce(func() {
		ignoredFilesProgressCurrentPath.Refresh()
	}, time.Second/60)
	var ignoredFilesCtxStop context.CancelFunc
	reScanIgnoredFiles := func() error {
		var ignoredFilesCtx context.Context
		ignoredFilesCtx, ignoredFilesCtxStop = context.WithCancel(guiCtx)

		ignoredFilesProgressBar.SetValue(0)
		ignoredFilesProgressCurrentDropboxPath.SetText("")
		ignoredFilesProgress.Show()
		ignoredFileNames.RemoveAll()

		for i, dropboxIgnorer := range dropboxIgnorers {
			ignoredFilesProgressCurrentDropboxPath.SetText(dropboxIgnorer.dropboxPath)

			err := filepath.WalkDir(dropboxIgnorer.dropboxPath, func(path string, info fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				err = ignoredFilesCtx.Err()
				if err != nil {
					return fmt.Errorf("ctx canceled: %s", err)
				}
				ignoredFilesProgressCurrentPath.Text = path
				ignoredFilesProgressCurrentPathRefreshDebounced()
				isIgnored, err := HasDropboxIgnoreFlag(path)
				if err != nil {
					return err
				}
				if isIgnored && !ignoredFileNames.Has(path) && !strings.HasPrefix(path, filepath.Join(dropboxIgnorer.dropboxPath, ".dropbox.cache")) {
					ignoredFileNames.Add(path)
				}

				return nil
			})
			if err != nil {
				return err
			}
			ignoredFilesProgressBar.SetValue(float64(i + 1))
		}
		ignoredFilesProgress.Hide()

		return nil
	}
	ignoredFilesContentError := widget.NewLabel("")
	ignoredFilesContentError.Hide()
	toggleShowOnlyRemovableOrAllFilesButton := widget.NewCheck("Show only unignoreable", func(value bool) {
		showOnlyRemovableOrAllFiles = value
		ignoredFilesListContent.Refresh()
	})
	toggleShowOnlyRemovableOrAllFilesButton.Checked = showOnlyRemovableOrAllFiles

	unignoreSelectedPaths := func() {
		var errTest []string
		for _, name := range checkedFileNames.Values() {
			err := RemoveDropboxIgnoreFlag(name)
			if err != nil {
				log.Printf("error removing ignore flag from path %s: %s", name, err)
				errTest = append(errTest, fmt.Sprintf("error removing ignore flag from path %s: %s", name, err))
			} else {
				ignoredPathsSet.Remove(name)
				ignoredFileNames.Remove(name)
				checkedFileNames.Remove(name)
			}
		}
		if len(errTest) > 0 {
			ignoredFilesContentError.SetText(strings.Join(errTest, "\n"))
			ignoredFilesContentError.Show()
		}
	}
	ignoredFilesRemoveIgnoreFlagButton := widget.NewButton("", func() {
		confirmDialog := dialog.NewConfirm("Unignore", fmt.Sprintf("Are you sure to unignore %d paths?", checkedFileNames.Len()), func(b bool) {
			if b {
				unignoreSelectedPaths()
			}
		}, w)
		confirmDialog.Show()
	})
	updateIgnoredFilesRemoveIgnoreFlagButton := func() {
		count := checkedFileNames.Len()
		ignoredFilesRemoveIgnoreFlagButton.SetText(fmt.Sprintf("(%d) Unignore", count))
		if count == 0 {
			ignoredFilesRemoveIgnoreFlagButton.Disable()
		} else {
			ignoredFilesRemoveIgnoreFlagButton.Enable()
		}
	}
	checkedFileNames.AddChangeEventListener(updateIgnoredFilesRemoveIgnoreFlagButton)
	updateIgnoredFilesRemoveIgnoreFlagButton()

	ignoredFilesContent := container.NewBorder(
		container.NewVBox(ignoredFilesProgress, ignoredFilesContentError),
		container.NewHBox(toggleShowOnlyRemovableOrAllFilesButton, ignoredFilesRemoveIgnoreFlagButton),
		nil, nil,
		ignoredFilesListContent,
	)
	ignoredFilesTab := container.NewTabItemWithIcon("Ignored Files", theme.VisibilityOffIcon(), ignoredFilesContent)

	for _, d := range dropboxIgnorers {
		ignoreFilesSet.Add(filepath.Join(d.dropboxPath, DropboxIgnoreFilename))
	}
	ignoreFilesSetList := widget.NewList(
		func() int {
			return ignoreFilesSet.Len()
		},
		func() fyne.CanvasObject {
			var button *widget.Button
			button = widget.NewButton("", func() {
				path := button.Text

				// create file if it not exists
				_, err := os.Stat(path)
				if err != nil {
					if !os.IsNotExist(err) {
						log.Printf("Error checking if path %s exists: %s", path, err)
						return
					} else {
						err := os.WriteFile(path, []byte{}, os.ModePerm)
						if err != nil {
							log.Printf("Error creating dropbox ignore file %s: %s", path, err)
						}
					}
				}

				/*
					err := open.Run(path)
					if err != nil {
						log.Printf("Error open path %s: %s", path, err)
					}
				*/
				// open file in explorer
				// https://askubuntu.com/questions/133597/reveal-file-in-file-explorer
				var cmd *exec.Cmd
				switch runtime.GOOS {
				case "windows":
					cmd = exec.Command("explorer", "/select,", path)
				case "darwin":
					cmd = exec.Command("open", "-R", path)
				case "linux":
					// cSpell: words: dbus freedesktop
					cmd = exec.Command("dbus-send", "--session", "-dest=org.freedesktop.FileManager1", "--type=method_call", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems", `array:string:"`+strings.ReplaceAll(path, "\"", "\\\"")+`"`, `string:""`)
				default:
					cmd = exec.Command("xdg-open", path)
				}
				cmd.Run() //nolint:errcheck

				/* ignore error, windows explorer always returns 1 as exit status
				// err := cmd.Run()
				out, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("Error open path: %s", err)
					log.Printf("Program output: %s", string(out))
				}
				*/
			})
			return button
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			// multithreading
			name := ignoreFilesSet.GetOrEmptyString(i)

			button := o.(*widget.Button)
			button.SetText(name)
		},
	)
	ignoreFilesSet.AddChangeEventListener(Debounce(func() {
		ignoreFilesSetList.Refresh()
	}, time.Second/60))
	dropboxIgnoreFileContent := container.NewBorder(
		nil, nil, nil, nil,
		ignoreFilesSetList,
	)
	dropboxIgnoreFileTab := container.NewTabItemWithIcon(".dropboxignore File", theme.FileTextIcon(), dropboxIgnoreFileContent)

	logStringSliceList := widget.NewList(
		func() int {
			return len(logStringSlice.data)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			values := logStringSlice.data
			i = len(values) - i - 1
			data := ""
			if i < len(values) {
				data = values[i]
			}

			label := o.(*widget.Label)
			label.SetText(data)
		},
	)
	logStringSliceListRefreshDebounced := Debounce(func() {
		logStringSliceList.Refresh()
	}, time.Second/60)
	logStringSlice.AddChangeEventListener(func() {
		logStringSliceListRefreshDebounced()
	})
	var logsCopyButton *widget.Button
	logsCopyButton = widget.NewButton("Copy Log to clipboard", func() {
		w.Clipboard().SetContent(logStringSlice.String())

		bakText := logsCopyButton.Text
		logsCopyButton.SetText(bakText + " copied!")
		go func() {
			time.Sleep(3 * time.Second)
			logsCopyButton.SetText(bakText)
		}()

	})
	logsContent := container.NewBorder(
		nil,
		logsCopyButton,
		nil, nil,
		logStringSliceList,
	)
	logsTab := container.NewTabItemWithIcon("Logs", theme.FileTextIcon(), logsContent)

	autostartEnabled, err := IsAutoStartEnabled()
	if err != nil {
		return fmt.Errorf("error checking if autostart is enabled: %w", err)
	}
	var autoStartCheckBox *widget.Check
	autoStartCheckBox = widget.NewCheck("Autostart", func(value bool) {
		var err error
		if value {
			err = EnableAutoStart()
			if err != nil {
				log.Printf("error enable autostart: %s", err)
			}
		} else {
			err = DisableAutoStart()
			if err != nil {
				log.Printf("error disable autostart: %s", err)
			}
		}
		if err != nil {
			autoStartCheckBox.SetChecked(!value)
		}
	})
	autoStartCheckBox.SetChecked(autostartEnabled)
	quitButton := widget.NewButtonWithIcon("Quit Application", theme.LogoutIcon(), func() {
		log.Printf("quit button clicked")
		a.Quit()
	})
	settingsContent := container.NewBorder(
		nil,
		quitButton,
		nil, nil,
		container.NewVBox(
			autoStartCheckBox,
			container.NewBorder(
				nil,
				widget.NewLabel(appNameToUserDisplay(a)+" "+a.Metadata().Version),
				nil, nil,
			),
		),
	)
	// settingsTab := container.NewTabItem("Settings", settingsContent)
	settingsTab := container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), settingsContent)

	tabs := container.NewAppTabs(
		homeTab,
		ignoredFilesTab,
		dropboxIgnoreFileTab,
		logsTab,
		settingsTab,
	)

	if desk, ok := a.(desktop.App); ok {
		var m *fyne.Menu = fyne.NewMenu(appNameToUserDisplay(a),
			fyne.NewMenuItem("Show", func() {
				w.Show()
			}),
			fyne.NewMenuItem("Ignored Files", func() {
				tabs.Select(ignoredFilesTab)
				w.Show()
			}),
			fyne.NewMenuItem("Logs", func() {
				tabs.Select(logsTab)
				w.Show()
			}),
			fyne.NewMenuItem("Settings", func() {
				tabs.Select(settingsTab)
				w.Show()
			}),
			fyne.NewMenuItem("Quit", func() {
				a.Quit()
			}),
		)
		desk.SetSystemTrayMenu(m)
		systray.SetTitle(m.Label)
		systray.SetTooltip(m.Label)
	}

	tabs.OnSelected = func(ti *container.TabItem) {
		if ti == ignoredFilesTab {
			ignoredFilesContentError.Hide()
			go func() {
				err := reScanIgnoredFiles()
				if err != nil {
					log.Printf("Error scanning files: %s", err)
					ignoredFilesContentError.SetText(fmt.Sprintf("error scanning files: %s", err))
					ignoredFilesContentError.Show()
				}
			}()
		} else {
			if ignoredFilesCtxStop != nil {
				ignoredFilesCtxStop()
			}
		}
	}
	w.SetContent(tabs)

	// SetCloseIntercept => will hide the application instead of closing it
	w.SetCloseIntercept(func() {
		log.Printf("Close intercept: hide window")
		w.Hide()
	})
	go func() {
		<-guiCtx.Done()
		a.Quit()
	}()

	if hideGUI {
		// run only launches the application without showing window
		a.Run()
	} else {
		// launches the application ans shows the window
		w.ShowAndRun()
	}

	return nil
}

func ShowError(errorText string) {
	a := app.New()
	w := a.NewWindow(appNameToUserDisplay(a))

	var copyErrorButton *widget.Button
	copyErrorButton = widget.NewButton("Copy Error to clipboard", func() {
		w.Clipboard().SetContent(errorText)

		bakText := copyErrorButton.Text
		copyErrorButton.SetText("copied!")
		go func() {
			time.Sleep(3 * time.Second)
			copyErrorButton.SetText(bakText)
		}()

	})
	content := container.NewBorder(
		nil,
		copyErrorButton,
		nil,
		nil,
		widget.NewLabel(errorText),
	)
	w.SetContent(content)

	w.ShowAndRun()
}
