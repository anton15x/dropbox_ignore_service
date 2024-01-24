package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
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
)

func ShowGUI(ctx context.Context, dropboxIgnorers []*DropboxIgnorer, ignoredPathsSet *SortedStringSet, ignoreFilesSet *SortedStringSet) error {
	// guiCtx, guiCtxStop := context.WithCancel(ctx)
	guiCtx := ctx
	// wrappErrFunction := func(f func() error) func() {
	// 	return func() {
	// 		err := f()
	// 		if err != nil {
	// 			log.Printf("errlr happend: %s", err)
	// 			guiCtxStop()
	// 		}
	// 	}
	// }

	// fyne bundle  --help
	//go:generate fyne bundle -o bundled_icon_generated.go assets/icon.png
	//go:generate fyne bundle -o bundled_icon_generated.go -append assets/icon.ico

	a := app.New()
	// a := app.NewWithID("dropbox_ignore_service")
	a.SetIcon(resourceIconIco)
	w := a.NewWindow("Hello World")
	w.Resize(fyne.NewSize(1200, 800))

	// w.SetContent(widget.NewLabel("Hello World!"))

	label1 := widget.NewLabel("Label 1")
	value1 := widget.NewEntry()
	value1.SetText("defaultvalue")

	label2 := widget.NewLabel("Label 2")
	value2 := widget.NewEntry()
	value2.SetPlaceHolder("placeholder")
	grid := container.New(layout.NewFormLayout(), label1, value1, label2, value2)

	w.SetContent(container.New(layout.NewFormLayout(), widget.NewLabel("Hello World2!"), grid))

	// container.NewTabItem("myform", myForm)

	// tab := container.NewTabItem("Tab 1", widget.NewLabel("Hello"))
	ignoredPathsSetList := widget.NewList(
		func() int {
			return len(ignoredPathsSet.Values)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			// multithreading
			values := ignoredPathsSet.Values
			name := ""
			if i < len(values) {
				name = values[i]
			}

			label := o.(*widget.Label)
			label.SetText(name)
		},
	)
	homeTopLabel := widget.NewLabel("")
	updateHomeTopLabel := func() {
		homeTopLabel.SetText(fmt.Sprintf("Ignoring %d elements", len(ignoredPathsSet.Values)))
	}
	ignoredPathsSet.AddChangeEventListener(updateHomeTopLabel)
	updateHomeTopLabel()
	homeContent := container.NewBorder(
		homeTopLabel,
		nil, nil, nil,
		ignoredPathsSetList,
	)
	homeTab := container.NewTabItemWithIcon("Home", theme.SettingsIcon(), homeContent)

	showOnlyRemoveableOrAllFiles := true
	ignoredFileNames := NewSortedStringSet()
	checkedFileNames := NewSortedStringSet()
	ignoredFileNamesValuesLastLenCall := []string{}
	ignoredFilesListContent := widget.NewList(
		func() int {
			// saving values makes it multihtreading safe
			ignoredFileNamesValuesLastLenCall = []string{}
			for _, val := range ignoredFileNames.Values {
				if !showOnlyRemoveableOrAllFiles || !ignoredPathsSet.Has(val) {
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

	ignoredFilesListContentRefreshDebounced := debounce(func() {
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
	ignoredFilesProgressCurrentPathRefreshDebounced := debounce(func() {
		ignoredFilesProgressCurrentPath.Refresh()
	}, time.Second/60)
	var ignoredFilesCtxStop context.CancelFunc
	rescanIgnoredFiles := func() error {
		var ignoredFilesCtx context.Context
		ignoredFilesCtx, ignoredFilesCtxStop = context.WithCancel(guiCtx)

		ignoredFilesProgressBar.SetValue(0)
		ignoredFilesProgressCurrentDropboxPath.SetText("")
		ignoredFilesProgress.Show()
		ignoredFileNames.RemoveAll()

		for i, dropboxIgnorer := range dropboxIgnorers {
			ignoredFilesProgressCurrentDropboxPath.SetText(dropboxIgnorer.dropboxPath)

			err := filepath.Walk(dropboxIgnorer.dropboxPath, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					return err
				}
				err = ignoredFilesCtx.Err()
				if err != nil {
					return fmt.Errorf("ctx cancled: %s", err)
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
	toogleShowOnlyRemoveableOrAllFilesButton := widget.NewCheck("Show only unignoreable", func(value bool) {
		showOnlyRemoveableOrAllFiles = value
		ignoredFilesListContent.Refresh()
	})
	toogleShowOnlyRemoveableOrAllFilesButton.Checked = showOnlyRemoveableOrAllFiles

	unignoreSelectedPaths := func() {
		var errTest []string
		for _, name := range checkedFileNames.Values {
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
		confirmDialog := dialog.NewConfirm("Unignore", fmt.Sprintf("Are you sure to unignore %d paths?", len(checkedFileNames.Values)), func(b bool) {
			if b {
				unignoreSelectedPaths()
			}
		}, w)
		confirmDialog.Show()
	})
	updateIgnoredFilesRemoveIgnoreFlagButton := func() {
		count := len(checkedFileNames.Values)
		ignoredFilesRemoveIgnoreFlagButton.SetText(fmt.Sprintf("(%d) Unignore", count))
		if count == 0 {
			ignoredFilesRemoveIgnoreFlagButton.Disable()
		} else {
			ignoredFilesRemoveIgnoreFlagButton.Enable()
		}
	}
	checkedFileNames.AddChangeEventListener(updateIgnoredFilesRemoveIgnoreFlagButton)
	updateIgnoredFilesRemoveIgnoreFlagButton()

	// ignoredFilesContent := container.New(layout.NewAdaptiveGridLayout(), ignoredFilesListContent, ignoredFilesContentError)
	// ignoredFilesContent := container.NewGridWithColumns(
	// 	1,
	ignoredFilesContent := container.NewBorder(
		container.NewVBox(ignoredFilesProgress, ignoredFilesContentError),
		container.NewHBox(toogleShowOnlyRemoveableOrAllFilesButton, ignoredFilesRemoveIgnoreFlagButton),
		nil, nil,
		ignoredFilesListContent,
	)
	ignoredFilesTab := container.NewTabItemWithIcon("Ignored Files", theme.VisibilityOffIcon(), ignoredFilesContent)

	ignoreFilesSetList := widget.NewList(
		func() int {
			return len(ignoreFilesSet.Values)
		},
		func() fyne.CanvasObject {
			var button *widget.Button
			button = widget.NewButton("", func() {
				path := button.Text
				/*
					err := open.Run(path)
					if err != nil {
						log.Printf("Error open path %s: %s", path, err)
					}
				*/
				var cmd *exec.Cmd
				// https://askubuntu.com/questions/133597/reveal-file-in-file-explorer
				switch runtime.GOOS {
				case "windows":
					cmd = exec.Command("explorer", "/select,", path)
				case "darwin":
					cmd = exec.Command("open", "-R", path)
				case "linux":
					cmd = exec.Command("dbus-send", "--session", "-dest=org.freedesktop.FileManager1", "--type=method_call", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems", `array:string:"`+strings.ReplaceAll(path, "\"", "\\\"")+`"`, `string:""`)
				default:
					cmd = exec.Command("xdg-open", path)
				}
				cmd.Run()

				/* ignore error, windows explorer always returns 1 as exit status
				// err := cmd.Run()
				out, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("Error open path: %s\nprogramm output: %s", err, string(out))
				}
				*/
			})
			return button
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			// multithreading
			values := ignoreFilesSet.Values
			name := ""
			if i < len(values) {
				name = values[i]
			}

			button := o.(*widget.Button)
			button.SetText(name)
		},
	)
	ignoreFilesSet.AddChangeEventListener(func() {
		ignoreFilesSetList.Refresh()
	})
	dropboxIgnoreFileContent := container.NewBorder(
		nil, nil, nil, nil,
		ignoreFilesSetList,
	)
	dropboxIgnoreFileTab := container.NewTabItemWithIcon(".dropboxignore File", theme.FileTextIcon(), dropboxIgnoreFileContent)

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
	quitButton := widget.NewButtonWithIcon("Quit Applicaiton", theme.LogoutIcon(), func() {
		log.Printf("quit button clicked")
		a.Quit()
	})
	settingsContent := container.NewBorder(
		nil,
		quitButton,
		nil, nil,
		container.NewVBox(
			autoStartCheckBox,
		),
	)
	// settingsTab := container.NewTabItem("Settings", settingsContent)
	settingsTab := container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), settingsContent)

	tabs := container.NewAppTabs(
		homeTab,
		ignoredFilesTab,
		dropboxIgnoreFileTab,
		settingsTab,
	)

	if desk, ok := a.(desktop.App); ok {
		var m *fyne.Menu = fyne.NewMenu("MyApp",
			fyne.NewMenuItem("Show", func() {
				w.Show()
			}),
			fyne.NewMenuItem("Ignored Files", func() {
				tabs.Select(ignoredFilesTab)
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
	}

	tabs.OnSelected = func(ti *container.TabItem) {
		if ti == ignoredFilesTab {
			ignoredFilesContentError.Hide()
			go func() {
				err := rescanIgnoredFiles()
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
	// tabs.SetTabLocation(container.TabLocationLeading)
	//tabs.OnSelected()
	w.SetContent(tabs)

	// SetCloseIntercept => will hide the applicaiton insted of closing it
	w.SetCloseIntercept(func() {
		w.Hide()
	})
	go func() {
		<-guiCtx.Done()
		a.Quit()
	}()
	go func() {
		time.Sleep(time.Second)
		w.Hide()
	}()
	a.Run()
	// w.ShowAndRun()

	return nil
}
