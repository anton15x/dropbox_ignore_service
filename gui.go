package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	"github.com/anton15x/dropbox_ignore_service/src/filewalkfast"
	"github.com/anton15x/dropbox_ignore_service/src/util"
	"github.com/c2h5oh/datasize"
	fynetooltip "github.com/dweymouth/fyne-tooltip"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
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

func ShowGUI(ctx context.Context, dropboxIgnorers []*DropboxIgnorer, hideGUI bool, ignoredPathsSet *util.SortedStringSet, ignoreFilesSet *util.SortedStringSet, logStringSlice *logStringSliceStruct) error {
	guiCtx, guiCtxCancel := context.WithCancel(ctx)
	defer guiCtxCancel()

	// FyneApp.toml has id and icon set => fyne build adds metadata for us
	// do not use go build, instead use:
	// fyne package --release
	a := app.New()
	// a := app.NewWithID("dropbox_ignore_service")
	// w := a.NewWindow(a.Metadata().Name)
	w := a.NewWindow(appNameToUserDisplay(a))
	w.Resize(fyne.NewSize(1200, 800))

	type ExtendedFileInfo struct {
		Size datasize.ByteSize
		Err  error
	}
	ignoresFileExtendedInfos := util.NewSafeMap[string, ExtendedFileInfo]()

	ignoredPathsSetList := widget.NewList(
		func() int {
			return ignoredPathsSet.Len()
		},
		func() fyne.CanvasObject {
			nameLabel := widget.NewLabel("")
			return NewRightClickableContainer(container.NewBorder(
				nil,
				nil,
				nil,
				widget.NewLabel(""), // size
				nameLabel,           // name
			), func(e *fyne.PointEvent, c *RightClickableContainer) {
				RightClickOpenPathActionsContextMenu(nameLabel.Text, e, c)
			})
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			// multithreading
			name := ignoredPathsSet.GetOrEmptyString(i)
			info, ok := ignoresFileExtendedInfos.Load(name)

			rightClickContainer := o.(*RightClickableContainer)
			container := rightClickContainer.Content.(*fyne.Container)
			label := container.Objects[0].(*widget.Label)
			sizeLabel := container.Objects[1].(*widget.Label)

			changed := false
			if label.Text != name {
				changed = true
				label.SetText(name)
			}

			var sizeLabelText string
			if !ok {
				sizeLabelText = "?"
			} else if info.Err != nil {
				sizeLabelText = info.Err.Error()
			} else {
				sizeLabelText = info.Size.HumanReadable()
			}
			if sizeLabel.Text != sizeLabelText {
				changed = true
				sizeLabel.SetText(sizeLabelText)
			}

			if changed {
				// if the text size changes, the container also must update
				container.Refresh()
			}
		},
	)
	homeTopLabel := widget.NewLabel("")
	updateHomeTopLabel := func() {
		homeTopLabel.SetText(fmt.Sprintf("Ignoring %d elements", ignoredPathsSet.Len()))
	}
	ignoredPathsListRefreshDebounced := Debounce(func() {
		FyneDoSync(a, func() {
			updateHomeTopLabel()
			ignoredPathsSetList.Refresh()
		})
	}, time.Second/60)
	ignoredPathsSet.AddChangeEventListener(ignoredPathsListRefreshDebounced)
	updateHomeTopLabel()
	refreshIgnoredPathsSet := widget.NewButtonWithIcon("refresh sizes", theme.ViewRefreshIcon(), nil)
	homeContent := container.NewBorder(
		container.NewBorder(
			nil,
			nil,
			homeTopLabel,
			refreshIgnoredPathsSet,
		),
		nil, nil, nil,
		ignoredPathsSetList,
	)
	homeTab := container.NewTabItemWithIcon("Home", theme.HomeIcon(), homeContent)

	showOnlyRemovableOrAllFiles := true
	ignoredFileNames := util.NewSortedStringSet()
	checkedFileNames := util.NewSortedStringSet()
	ignoredFileNamesValuesLastLenCall := []string{}
	var unignoreSelectedPaths func(paths []string)

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
			nameLabel := widget.NewLabel("")
			check := widget.NewCheck("", func(b bool) {
				name := nameLabel.Text
				if b {
					checkedFileNames.Add(name)
				} else {
					checkedFileNames.Remove(name)
				}
			})
			rightClickContainer := NewRightClickableContainer(container.NewBorder(
				nil,
				nil,
				check,
				widget.NewLabel(""), // size
				nameLabel,           // name
			), func(e *fyne.PointEvent, c *RightClickableContainer) {
				name := nameLabel.Text

				button := widget.NewButton("Unignore", func() {
					HidePopup()
					confirmDialog := dialog.NewConfirm("Unignore", fmt.Sprintf("Are you sure to unignore the path ? %s", name), func(b bool) {
						if b {
							unignoreSelectedPaths([]string{name})
						}
					}, w)
					confirmDialog.Show()
				})
				if check.Disabled() {
					// if file gets unignoreable, the button does not get updated
					// unignoreSelectedPaths will show an error to the user, that the file is still actively ignored, and
					// that it is an dropbox_ignore_service error and not a system error
					button.Disable()
				}
				RightClickOpenPathActionsContextMenu(name, e, c, button)
			})
			rightClickContainer.OnLeftClick = func(e *fyne.PointEvent, c *RightClickableContainer) {
				if !check.Disabled() {
					check.SetChecked(!check.Checked)
				}
			}
			return rightClickContainer
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			values := ignoredFileNamesValuesLastLenCall
			name := ""
			if i < len(values) {
				name = values[i]
			}
			info, ok := ignoresFileExtendedInfos.Load(name)

			rightClickContainer := o.(*RightClickableContainer)
			container := rightClickContainer.Content.(*fyne.Container)
			nameLabel := container.Objects[0].(*widget.Label)
			check := container.Objects[1].(*widget.Check)
			sizeLabel := container.Objects[2].(*widget.Label)

			changed := false
			if check.Checked != checkedFileNames.Has(name) {
				check.SetChecked(!check.Checked)
			}

			importance := widget.MediumImportance
			if ignoredPathsSet.Has(name) {
				importance = widget.LowImportance
				check.Partial = true
				check.Disable()
			} else {
				check.Partial = false
				check.Enable()
			}

			if nameLabel.Text != name {
				changed = true
				nameLabel.SetText(name)
			}
			if nameLabel.Importance != importance {
				changed = true
				nameLabel.Importance = importance
			}

			var sizeLabelText string
			if !ok {
				sizeLabelText = "?"
			} else if info.Err != nil {
				sizeLabelText = info.Err.Error()
			} else {
				sizeLabelText = info.Size.HumanReadable()
			}
			if sizeLabel.Text != sizeLabelText {
				changed = true
				sizeLabel.SetText(sizeLabelText)
			}

			if changed {
				// if the text size changes, the container also must update
				container.Refresh()
			}
		},
	)

	ignoredFilesListContentRefreshDebounced := Debounce(func() {
		FyneDoSync(a, func() {
			ignoredFilesListContent.Refresh()
		})
	}, time.Second/60)

	ignoredFilesProgressBar := widget.NewProgressBar()
	ignoredFilesProgressBar.Max = float64(len(dropboxIgnorers))
	ignoredFilesProgressCurrentDropboxPath := widget.NewLabel("")
	ignoredFilesProgressCurrentDropboxPath.Wrapping = fyne.TextWrapWord
	ignoredFilesProgressCurrentPath := widget.NewLabel("")
	ignoredFilesProgressCurrentPath.Wrapping = fyne.TextWrapWord
	ignoredFilesProgress := container.NewVBox(
		ignoredFilesProgressBar,
		container.NewBorder(nil, nil, widget.NewLabel("current dropbox root:"), nil, ignoredFilesProgressCurrentDropboxPath),
		container.NewBorder(nil, nil, widget.NewLabel("current path:"), nil, ignoredFilesProgressCurrentPath),
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
	ignoredFilesProgressCurrentPathRefreshDebounced := DebounceVal(func(val string) {
		FyneDoSync(a, func() {
			ignoredFilesProgressCurrentPath.Text = val
			ignoredFilesProgressCurrentPath.Refresh()
		})
	}, time.Second/60)
	var ignoredFilesCtxStopError = errors.New("ignoredFilesCtxStopped")
	scanIgnoredFilesBase := func(parent context.Context) error {
		startTime := time.Now()
		ctx, cancel := context.WithCancel(parent)
		defer cancel()

		FyneDoSync(a, func() {
			ignoredFilesProgressBar.SetValue(0)
			ignoredFilesProgressCurrentDropboxPath.SetText("")
			ignoredFilesProgress.Show()
		})
		ignoredFileNames.RemoveAll()

		for i, dropboxIgnorer := range dropboxIgnorers {
			FyneDoSync(a, func() {
				ignoredFilesProgressCurrentDropboxPath.SetText(dropboxIgnorer.dropboxPath)
			})

			type Work struct {
				Path string
			}
			work := make(chan Work, 100000)
			var wg sync.WaitGroup
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for w := range work {
						path := w.Path
						isIgnored, err := HasDropboxIgnoreFlag(path)
						if err != nil {
							log.Printf("error checking %s: %s", path, err)
						}
						if isIgnored && !ignoredFileNames.Has(path) && !strings.HasPrefix(path, filepath.Join(dropboxIgnorer.dropboxPath, ".dropbox.cache")) {
							ignoredFileNames.Add(path)
						}
					}
				}()
			}

			err := filewalkfast.WalkUnorderedMemoryAware(dropboxIgnorer.dropboxPath, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if err = ctx.Err(); err != nil {
					return errors.Join(ignoredFilesCtxStopError, err)
				}

				if !info.IsDir() {
					if !info.Mode().IsRegular() {
						// only files/directories may have the ignore flag, but not symlinks
						return nil
					}
				}

				ignoredFilesProgressCurrentPathRefreshDebounced(path)
				work <- Work{
					Path: path,
				}

				return nil
			})
			if err != nil {
				return err
			}
			close(work)
			wg.Wait()

			FyneDoSync(a, func() {
				ignoredFilesProgressBar.SetValue(float64(i + 1))
			})
		}
		FyneDoSync(a, func() {
			ignoredFilesProgress.Hide()
		})

		// no status output: 10:07:24 scanning took 10.8792017s
		duration := time.Since(startTime)
		log.Printf("scanning took %s", duration.String())
		return nil
	}
	ignoredFilesContentError := widget.NewLabel("")
	ignoredFilesContentError.Hide()
	scanIgnoredFiles := util.SingleExecutionContextStopCtx(func(ctx context.Context) {
		FyneDoSync(a, func() {
			ignoredFilesContentError.Hide()
		})
		err := scanIgnoredFilesBase(ctx)
		if err != nil && !errors.Is(err, ignoredFilesCtxStopError) {
			FyneDoSync(a, func() {
				log.Printf("Error scanning files: %s", err)
				ignoredFilesContentError.SetText(fmt.Sprintf("error scanning files: %s", err))
				ignoredFilesContentError.Show()
			})
		}
	})
	toggleShowOnlyRemovableOrAllFilesButton := widget.NewCheck("Show only unignoreable", func(value bool) {
		showOnlyRemovableOrAllFiles = value
		ignoredFilesListContent.Refresh()
	})
	toggleShowOnlyRemovableOrAllFilesButton.Checked = showOnlyRemovableOrAllFiles

	unignoreSelectedPaths = func(paths []string) {
		var errText []string
		for _, name := range paths {
			var err error
			if ignoredPathsSet.Has(name) {
				err = fmt.Errorf("file is actively ignored and may not get unignored (dropbox_ignore_service error)")
			} else {
				err = RemoveDropboxIgnoreFlag(name)
			}
			if err != nil {
				log.Printf("error removing ignore flag from path %s: %s", name, err)
				errText = append(errText, fmt.Sprintf("error removing ignore flag from path %s: %s", name, err))
			} else {
				ignoredPathsSet.Remove(name)
				ignoredFileNames.Remove(name)
				checkedFileNames.Remove(name)
			}
		}
		if len(errText) > 0 {
			ignoredFilesContentError.SetText(strings.Join(errText, "\n"))
			ignoredFilesContentError.Show()
		}
	}
	ignoredFilesRemoveIgnoreFlagButton := widget.NewButton("", func() {
		confirmDialog := dialog.NewConfirm("Unignore", fmt.Sprintf("Are you sure to unignore %d paths?", checkedFileNames.Len()), func(b bool) {
			if b {
				unignoreSelectedPaths(checkedFileNames.Values())
			}
		}, w)
		confirmDialog.Show()
	})
	updateIgnoredFilesRemoveIgnoreFlagButton := Debounce(func() {
		FyneDoSync(a, func() {
			count := checkedFileNames.Len()
			ignoredFilesRemoveIgnoreFlagButton.SetText(fmt.Sprintf("(%d) Unignore", count))
			if count == 0 {
				ignoredFilesRemoveIgnoreFlagButton.Disable()
			} else {
				ignoredFilesRemoveIgnoreFlagButton.Enable()
			}
		})
	}, time.Second/60)
	checkedFileNames.AddChangeEventListener(updateIgnoredFilesRemoveIgnoreFlagButton)
	updateIgnoredFilesRemoveIgnoreFlagButton()

	refreshButtonIgnoredFilesContentButton := refreshButton(false, func() {
		go scanIgnoredFiles(guiCtx)
	})

	ignoredFilesContent := container.NewBorder(
		container.NewVBox(ignoredFilesProgress, ignoredFilesContentError),
		container.NewHBox(refreshButtonIgnoredFilesContentButton, toggleShowOnlyRemovableOrAllFilesButton, ignoredFilesRemoveIgnoreFlagButton),
		nil, nil,
		ignoredFilesListContent,
	)
	ignoredFilesTabLoaded := false
	ignoredFilesTab := container.NewTabItemWithIcon("Ignored Files", theme.VisibilityOffIcon(), ignoredFilesContent)

	for _, d := range dropboxIgnorers {
		ignoreFilesSet.Add(filepath.Join(d.dropboxPath, DropboxIgnoreFilename))
	}
	ignoreFilesSetList := widget.NewList(
		func() int {
			return ignoreFilesSet.Len()
		},
		func() fyne.CanvasObject {
			button := widget.NewLabel("")
			return NewRightClickableContainer(button, func(e *fyne.PointEvent, c *RightClickableContainer) {
				RightClickOpenPathActionsContextMenu(button.Text, e, c)
			})
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			// multithreading
			name := ignoreFilesSet.GetOrEmptyString(i)

			rightClickContainer := o.(*RightClickableContainer)
			button := rightClickContainer.Content.(*widget.Label)

			button.SetText(name)
		},
	)
	ignoreFilesSet.AddChangeEventListener(Debounce(func() {
		FyneDoSync(a, func() {
			ignoreFilesSetList.Refresh()
		})
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
			label := widget.NewLabel("")

			return NewRightClickableContainer(label, func(e *fyne.PointEvent, c *RightClickableContainer) {
				text := label.Text

				textLabel := widget.NewLabel(text)
				textSize := textLabel.MinSize()
				textLabel.Wrapping = fyne.TextWrapWord
				textContent := NewMinSizeWrapper(textLabel, fyne.Size{Width: min(w.Canvas().Size().Width, textSize.Width)})

				button := widget.NewButton("Copy Text", func() {
					a.Clipboard().SetContent(text)
					HidePopup()
				})

				RightClickOpenContextMenu(e, c, textContent, button)
			})
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			values := logStringSlice.data
			i = len(values) - i - 1
			data := ""
			if i < len(values) {
				data = values[i]
			}

			rightClickContainer := o.(*RightClickableContainer)
			label := rightClickContainer.Content.(*widget.Label)

			label.SetText(data)
		},
	)
	logStringSliceListRefreshDebounced := Debounce(func() {
		FyneDoSync(a, func() {
			logStringSliceList.Refresh()
		})
	}, time.Second/60)
	logStringSlice.AddChangeEventListener(func() {
		logStringSliceListRefreshDebounced()
	})
	var logsCopyButton *widget.Button
	const logsCopyButtonText = "Copy Log to clipboard"
	logsCopyButton = widget.NewButton(logsCopyButtonText, func() {
		a.Clipboard().SetContent(logStringSlice.String())

		logsCopyButton.SetText(logsCopyButtonText + " copied!")
		go func() {
			time.Sleep(3 * time.Second)
			FyneDoSync(a, func() {
				logsCopyButton.SetText(logsCopyButtonText)
			})
		}()

	})
	logsContent := container.NewBorder(
		nil,
		logsCopyButton,
		nil, nil,
		logStringSliceList,
	)
	logsTab := container.NewTabItemWithIcon("Logs", theme.FileTextIcon(), logsContent)

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
			FyneDoSync(a, func() {
				autoStartCheckBox.SetChecked(!value)
			})
		}
	})
	refreshAutoStartCheckBox := func() {
		autostartEnabled, err := IsAutoStartEnabled()
		if err != nil {
			log.Printf("error checking if autostart is enabled: %s", err)
			return
		}
		FyneDoSync(a, func() {
			autoStartCheckBox.SetChecked(autostartEnabled)
		})
	}
	quitButton := widget.NewButtonWithIcon("Quit Application", theme.LogoutIcon(), func() {
		log.Printf("quit button clicked")
		guiCtxCancel()
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

	roots := make([]string, 0, len(dropboxIgnorers))
	for _, dropboxIgnorer := range dropboxIgnorers {
		roots = append(roots, dropboxIgnorer.dropboxPath)
	}
	fileTreeTabLoaded := false
	fileTree := newFileTree(a, roots, func(path string) bool {
		val, err := HasDropboxIgnoreFlag(path)
		return val && err == nil
	})
	fileTreeTab := container.NewTabItemWithIcon("FileTree", theme.ListIcon(), addFloatingRefresh(fileTree.Content(), func() {
		go fileTree.Scan(guiCtx)
	}))

	extendedInfoQueue := util.NewSafeMap[string, struct{}]()
	var enableExtendedInfoQ atomic.Bool
	var enableExtendedInfoQWorker atomic.Bool
	handleExtendedInfoQ := Debounce(func() {
		for enableExtendedInfoQWorker.Load() {
			foundWork := false
			for path := range extendedInfoQueue.Keys() {
				extendedInfoQueue.Delete(path)

				foundWork = true

				if !ignoredPathsSet.Has(path) && !ignoredFileNames.Has(path) {
					// obsolete
					continue
				}

				var extendedInfo ExtendedFileInfo
				err := filewalkfast.WalkUnorderedMemoryAware(path, func(path string, info fs.FileInfo, err error) error {
					if err != nil {
						return err
					}
					extendedInfo.Size += datasize.ByteSize(info.Size())
					return nil
				})
				if err != nil {
					extendedInfo.Err = err
				}

				// to avoid race conditions: store first, and delete after if not needed anymore
				// otherwise the "has"-check could return true and we store it, but in between the file got removed and the file extended info would be kept in memory.
				ignoresFileExtendedInfos.Store(path, extendedInfo)

				used := false
				if ignoredPathsSet.Has(path) {
					used = true
					ignoredPathsListRefreshDebounced()
				}
				if ignoredFileNames.Has(path) {
					used = true
					ignoredFilesListContentRefreshDebounced()
				}

				if !used {
					// got obsolete after scanning
					ignoresFileExtendedInfos.Delete(path)
					continue
				}
			}

			if !foundWork {
				return
			}
		}
	}, time.Second/60)
	ignoredPathsSet.AddAddEventListener(func(s string) {
		if enableExtendedInfoQ.Load() {
			extendedInfoQueue.Store(s, struct{}{})
			handleExtendedInfoQ()
		}
	})
	ignoredPathsSet.AddRemoveEventListener(func(s string) {
		if enableExtendedInfoQ.Load() && !ignoredFileNames.Has(s) {
			extendedInfoQueue.Delete(s)
			ignoresFileExtendedInfos.Delete(s)
		}
	})

	ignoredFileNames.AddAddEventListener(func(s string) {
		if enableExtendedInfoQ.Load() {
			extendedInfoQueue.Store(s, struct{}{})
			handleExtendedInfoQ()
		}
	})
	ignoredFileNames.AddRemoveEventListener(func(s string) {
		if enableExtendedInfoQ.Load() {
			extendedInfoQueue.Delete(s)
			ignoresFileExtendedInfos.Delete(s)
		}
	})
	refreshExtendedFileInfosDebounced := Debounce(func() {
		ignoresFileExtendedInfos.Clear()
		extendedInfoQueue.Clear()

		if !enableExtendedInfoQ.Load() {
			return
		}

		// remove duplicates first
		values := slices.Concat(ignoredPathsSet.Values(), ignoredFileNames.Values())
		slices.Sort(values)
		values = slices.Compact(values)

		for _, path := range values {
			extendedInfoQueue.Store(path, struct{}{})
		}
		handleExtendedInfoQ()
	}, 0)
	refreshIgnoredPathsSet.OnTapped = func() {
		refreshExtendedFileInfosDebounced()
	}

	tabs := container.NewAppTabs(
		homeTab,
		ignoredFilesTab,
		dropboxIgnoreFileTab,
		fileTreeTab,
		logsTab,
		settingsTab,
	)

	if desk, ok := a.(desktop.App); ok {
		openTabAndShow := func(item *container.TabItem) {
			if tabs.Selected() != item {
				tabs.Select(item)
			} else {
				// ensure to call selected callback after open window again
				tabs.OnSelected(item)
			}
			w.Show()
		}
		var m *fyne.Menu = fyne.NewMenu(appNameToUserDisplay(a),
			fyne.NewMenuItem("Show", func() {
				openTabAndShow(tabs.Selected())
			}),
			fyne.NewMenuItem("Home", func() {
				openTabAndShow(homeTab)
			}),
			fyne.NewMenuItem("Ignored Files", func() {
				openTabAndShow(ignoredFilesTab)
			}),
			fyne.NewMenuItem("Logs", func() {
				openTabAndShow(logsTab)
			}),
			fyne.NewMenuItem("Settings", func() {
				openTabAndShow(settingsTab)
			}),
			fyne.NewMenuItem("Quit", func() {
				guiCtxCancel()
			}),
		)
		desk.SetSystemTrayMenu(m)
		systray.SetTitle(m.Label)
		systray.SetTooltip(m.Label)
	}

	tabs.OnSelected = func(ti *container.TabItem) {
		if ti == homeTab || ti == ignoredFilesTab {
			enableExtendedInfoQWorker.Store(true)
			if enableExtendedInfoQ.CompareAndSwap(false, true) {
				refreshExtendedFileInfosDebounced()
			}
			handleExtendedInfoQ()
		} else {
			enableExtendedInfoQWorker.Store(false)
		}

		if ti == ignoredFilesTab {
			if !ignoredFilesTabLoaded {
				ignoredFilesTabLoaded = true
				go scanIgnoredFiles(guiCtx)
			}
		}

		if ti == fileTreeTab {
			if !fileTreeTabLoaded {
				fileTreeTabLoaded = true
				go fileTree.Scan(guiCtx)
			}
		}

		if ti == settingsTab {
			go refreshAutoStartCheckBox()
		}
	}
	tabs.OnSelected(tabs.Selected())

	w.SetContent(fynetooltip.AddWindowToolTipLayer(tabs, w.Canvas()))

	// SetCloseIntercept => will hide the application instead of closing it
	w.SetCloseIntercept(func() {
		log.Printf("Close intercept: hide window")
		w.Hide()

		if enableExtendedInfoQ.CompareAndSwap(true, false) {
			refreshExtendedFileInfosDebounced()
		}
		if fileTreeTabLoaded {
			fileTreeTabLoaded = false
			go fileTree.Clean()
		}
	})
	go func() {
		<-guiCtx.Done()
		FyneDoSync(a, func() {
			a.Quit()
		})
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

func refreshButton(small bool, onRefresh func()) fyne.CanvasObject {
	const text = "Rescan all files"
	if small {
		refreshBtn := ttwidget.NewButtonWithIcon("", theme.ViewRefreshIcon(), onRefresh)
		refreshBtn.SetToolTip(text)
		return refreshBtn
	}

	refreshBtn := widget.NewButtonWithIcon(text, theme.ViewRefreshIcon(), onRefresh)

	return refreshBtn
}

func addFloatingRefresh(c fyne.CanvasObject, onRefresh func()) fyne.CanvasObject {
	// Wrap the button in a bottom-right aligned container
	floating := container.New(
		layout.NewVBoxLayout(),
		layout.NewSpacer(),
		container.New(
			layout.NewHBoxLayout(),
			refreshButton(true, onRefresh),
			layout.NewSpacer(),
		),
	)

	// Overlay the button on top of the main container
	return container.New(layout.NewStackLayout(), c, floating)
}

func ShowError(errorText string) {
	a := app.New()
	w := a.NewWindow(appNameToUserDisplay(a))

	var copyErrorButton *widget.Button
	copyErrorButton = widget.NewButton("Copy Error to clipboard", func() {
		a.Clipboard().SetContent(errorText)

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

type MinSizeWrapper struct {
	widget.BaseWidget
	Child fyne.CanvasObject
	Min   fyne.Size
}

func NewMinSizeWrapper(child fyne.CanvasObject, min fyne.Size) *MinSizeWrapper {
	w := &MinSizeWrapper{
		Child: child,
		Min:   min,
	}
	w.ExtendBaseWidget(w)
	return w
}

func (w *MinSizeWrapper) CreateRenderer() fyne.WidgetRenderer {
	return &minSizeWrapperRenderer{
		w: w,
	}
}

type minSizeWrapperRenderer struct {
	w *MinSizeWrapper
}

func (r *minSizeWrapperRenderer) Layout(size fyne.Size) {
	r.w.Child.Resize(size)
}

func (r *minSizeWrapperRenderer) MinSize() fyne.Size {
	min := r.w.Child.MinSize()

	if min.Width < r.w.Min.Width {
		min.Width = r.w.Min.Width
	}
	if min.Height < r.w.Min.Height {
		min.Height = r.w.Min.Height
	}

	return min
}

func (r *minSizeWrapperRenderer) Refresh() {
	r.w.Child.Refresh()
}

func (r *minSizeWrapperRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.w.Child}
}

func (r *minSizeWrapperRenderer) Destroy() {}
