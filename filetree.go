package main

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"iter"
	"log"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/anton15x/dropbox_ignore_service/src/filewalkfast"
	"github.com/anton15x/dropbox_ignore_service/src/open"
	"github.com/anton15x/dropbox_ignore_service/src/util"
	"github.com/c2h5oh/datasize"
)

type FileTree struct {
	content fyne.CanvasObject

	clean func()
	scan  func(parent context.Context)
}

func (f *FileTree) Content() fyne.CanvasObject {
	return f.content
}
func (f *FileTree) Clean() {
	f.clean()
}
func (f *FileTree) Scan(parent context.Context) {
	f.scan(parent)
}

func newFileTree(myApp fyne.App, roots []string, shouldSkip func(string) bool) *FileTree {
	dummyEntry := Entry{
		Name: "dummy",
	}
	dummyIcon := theme.ErrorIcon()

	var scheduleRefresh func()

	m := util.NewSafeMap[string, *Entry]()
	searchM := util.NewSafeMap[string, *Entry]()
	var searchActive atomic.Bool
	getMap := func() *util.SafeMap[string, *Entry] {
		if searchActive.Load() {
			return searchM
		}
		return m
	}

	type headerEntry struct {
		Name                      string
		SortKey                   SortBy
		LayoutIdx                 int
		InitialSortOrderAscending bool
	}
	headers := []headerEntry{
		{
			Name:                      "Name",
			SortKey:                   SortByName,
			LayoutIdx:                 -1,
			InitialSortOrderAscending: true,
		},
		{
			Name:      "Files",
			SortKey:   SortByFileCount,
			LayoutIdx: 0,
		},
		{
			Name:      "Folders",
			SortKey:   SortByFolderCount,
			LayoutIdx: 1,
		},
		{
			Name:      "Size",
			SortKey:   SortBySize,
			LayoutIdx: 2,
		},
		{
			Name:      "Percentage",
			SortKey:   SortBySize,
			LayoutIdx: 3,
		},
		{
			Name:      "ModTime",
			SortKey:   SortByModTime,
			LayoutIdx: 4,
		},
	}
	separateFoldersAndFiles := true
	sortOrderAscending := false
	sortBySetting := SortBySize

	columnLayout := &ColumnLayout{RightToLeft: true}

	tree := widget.NewTree(
		func(id widget.TreeNodeID) []widget.TreeNodeID {
			curEntry, ok := getMap().Load(id)
			if !ok {
				return nil
			}

			ids := make([]widget.TreeNodeID, 0, len(curEntry.Folders)+len(curEntry.Files))

			// Sorting
			if separateFoldersAndFiles {
				entries := slices.Clone(curEntry.Folders)
				SortEntries(sortBySetting, entries, sortOrderAscending)
				for _, entry := range entries {
					ids = append(ids, entry.Name)
				}

				entries = append(entries[:0], curEntry.Files...)
				SortEntries(sortBySetting, entries, sortOrderAscending)
				for _, entry := range entries {
					ids = append(ids, entry.Name)
				}
			} else {
				entries := slices.Concat(curEntry.Folders, curEntry.Files)
				SortEntries(sortBySetting, entries, sortOrderAscending)
				for _, entry := range entries {
					ids = append(ids, entry.Name)
				}
			}

			return ids
		},
		func(id widget.TreeNodeID) bool {
			val, ok := getMap().Load(id)
			if !ok {
				return false
			}
			return val.IsDir
		},
		func(branch bool) fyne.CanvasObject {
			progress := widget.NewProgressBar()
			progress.TextFormatter = func() string { return fmt.Sprintf("%.2f%%", progress.Value) }
			progress.Min = 0
			progress.Max = 100
			progress.Value = 100

			return NewRightClickableContainer(container.NewBorder(
				nil,
				nil,
				container.NewHBox(
					// do not use widget.NewFileIcon, is calls os.Stat under the hood => bad performance
					// widget.NewFileIcon(storage.NewFileURI("/")),
					widget.NewIcon(dummyIcon),
					widget.NewLabel(""), // name
					widget.NewLabel(""), // error
				),
				container.New(
					columnLayout,
					widget.NewLabel(""), // files
					widget.NewLabel(""), // directories
					widget.NewLabel(""), // size
					progress,            // percent
					widget.NewLabel(""), // mod time
				),
			), nil)
		},
		func(id widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
			curEntry, ok := getMap().Load(id)
			if !ok {
				curEntry = &dummyEntry
			}

			text := filepath.Base(id)
			if curEntry.Parent == nil {
				// the root should get displayed with full path
				text = id
			}

			errorText := ""
			if curEntry.Error != nil {
				errorText = curEntry.Error.Error()
			}

			var icon fyne.Resource
			if curEntry.IsDir {
				icon = theme.FolderIcon()
			} else {
				icon = theme.FileIcon()
			}

			percentage := float64(curEntry.Size) / float64(curEntry.Root().Size) * 100

			rightClickContainer := o.(*RightClickableContainer)
			rootContainer := rightClickContainer.Content.(*fyne.Container)
			leftContainer := rootContainer.Objects[0].(*fyne.Container)
			rightContainer := rootContainer.Objects[1].(*fyne.Container)

			fileIcon := leftContainer.Objects[0].(*widget.Icon)
			nameLabel := leftContainer.Objects[1].(*widget.Label)
			errorLabel := leftContainer.Objects[2].(*widget.Label)

			filesLabel := rightContainer.Objects[0].(*widget.Label)
			dirsLabel := rightContainer.Objects[1].(*widget.Label)
			sizeLabel := rightContainer.Objects[2].(*widget.Label)
			sizeBar := rightContainer.Objects[3].(*widget.ProgressBar)
			modTimeLabel := rightContainer.Objects[4].(*widget.Label)

			rightClickContainer.OnRightClick = func(e *fyne.PointEvent, c *RightClickableContainer) {
				RightClickOpenPathActionsContextMenu(id, e, c)
			}

			UpdateContentValue(columnLayout, -1, fileIcon.SetResource, fileIcon, fileIcon.Resource, icon)
			UpdateContentValue(columnLayout, -1, nameLabel.SetText, nameLabel, nameLabel.Text, text)
			UpdateContentValue(columnLayout, -1, errorLabel.SetText, errorLabel, errorLabel.Text, errorText)

			UpdateContentValue(columnLayout, 0, filesLabel.SetText, filesLabel, filesLabel.Text, strconv.Itoa(curEntry.FileCount))
			UpdateContentValue(columnLayout, 1, dirsLabel.SetText, dirsLabel, dirsLabel.Text, strconv.Itoa(curEntry.FolderCount))
			UpdateContentValue(columnLayout, 2, sizeLabel.SetText, sizeLabel, sizeLabel.Text, curEntry.Size.HumanReadable())
			UpdateContentValue(columnLayout, 3, sizeBar.SetValue, sizeBar, sizeBar.Value, percentage)
			UpdateContentValue(columnLayout, 4, modTimeLabel.SetText, modTimeLabel, modTimeLabel.Text, curEntry.ModeTime.Format(time.DateOnly))
		},
	)

	headerButtons := make([]*widget.Button, len(headers))

	getHeaderTest := func(hh headerEntry, isSorted, sortAscending bool) string {
		if isSorted {
			if sortAscending {
				return hh.Name + " ↑"
			} else {
				return hh.Name + " ↓"
			}
		}

		return hh.Name
	}
	for i, header := range headers {
		button := widget.NewButton(header.Name, func() {
			// toggle sort direction if same column
			if sortBySetting == header.SortKey {
				sortOrderAscending = !sortOrderAscending
			} else {
				sortBySetting = header.SortKey
				sortOrderAscending = header.InitialSortOrderAscending
			}

			// update button labels with arrows
			for j, hh := range headers {
				text := getHeaderTest(hh, hh.SortKey == sortBySetting, sortOrderAscending)
				UpdateHeaderValue(columnLayout, hh.LayoutIdx, headerButtons[j].SetText, headerButtons[j], headerButtons[j].Text, text)
			}

			// rerender
			scheduleRefresh()
		})

		if header.LayoutIdx >= 0 {
			// calculate all button text variants to set maximum size
			button.SetText(getHeaderTest(header, true, true))
			columnLayout.SetHeaderWidth(header.LayoutIdx, button.MinSize())
			button.SetText(getHeaderTest(header, true, false))
			columnLayout.SetHeaderWidth(header.LayoutIdx, button.MinSize())

			button.SetText(getHeaderTest(header, false, false))
			columnLayout.SetHeaderWidth(header.LayoutIdx, button.MinSize())

			text := getHeaderTest(header, header.SortKey == sortBySetting, sortOrderAscending)
			UpdateHeaderValue(columnLayout, header.LayoutIdx, button.SetText, button, button.Text, text)
		}

		headerButtons[i] = button
	}

	separateFileAndFolders := widget.NewCheck("separate", func(b bool) {
		separateFoldersAndFiles = b

		scheduleRefresh()
	})
	separateFileAndFolders.Checked = separateFoldersAndFiles

	searchTextInput := widget.NewEntry()
	searchTextInput.SetPlaceHolder("Enter search text...")

	flattenCheckBox := widget.NewCheck("flatten", nil)
	regexCheckBox := widget.NewCheck("regex", nil)

	searchTextInput.Validator = func(s string) error {
		if regexCheckBox.Checked {
			_, err := regexp.Compile(s)
			if err != nil {
				return err
			}
		}

		return nil
	}

	updateSearch := Debounce(func() {
		val := searchTextInput.Text
		flatten := flattenCheckBox.Checked
		useRegex := regexCheckBox.Checked

		var valRegex *regexp.Regexp
		if val != "" {
			baseVal := val
			if !useRegex {
				baseVal = regexp.QuoteMeta(val)
			}
			var err error
			valRegex, err = regexp.Compile(baseVal)
			if err != nil {
				valRegex = nil
				log.Printf("error create regex of %s: %s", baseVal, err)
			}
		}

		isSearchActive := valRegex != nil || flatten
		searchActive.Store(isSearchActive)
		searchM.Clear()
		if isSearchActive {
			root, ok := m.Load("")
			if ok {
				var f func(originalEntry, p *Entry)
				f = func(originalEntry, p *Entry) {
					e := &Entry{
						Name:     originalEntry.Name,
						ModeTime: originalEntry.ModeTime,
						IsDir:    originalEntry.IsDir,
						Error:    originalEntry.Error,
						Parent:   p,
					}
					if !originalEntry.IsDir {
						e.Size = originalEntry.Size
					}

					matches := valRegex == nil || valRegex.MatchString(originalEntry.Name)
					if matches || e.IsDir {
						np := e
						if flatten && p != nil {
							np = p
						}

						for c := range ConcatSeq(originalEntry.Files, originalEntry.Folders) {
							f(c, np)
						}
					}

					if !matches && e.FileCount == 0 {
						return
					}
					if p != nil {
						p.FileCount += e.FileCount
						p.FolderCount += e.FolderCount
						if e.IsDir {
							p.Folders = append(p.Folders, e)
							p.FolderCount++
						} else {
							p.Files = append(p.Files, e)
							p.FileCount++
						}
						p.Size += e.Size
					}

					searchM.Store(e.Name, e)
				}
				f(root, nil)
			}
		}

		scheduleRefresh()
	}, time.Second/10)
	updateSearchIfActive := func() {
		if searchActive.Load() {
			updateSearch()
			return
		}

		scheduleRefresh()
	}

	flattenCheckBox.OnChanged = func(b bool) {
		updateSearch()
	}
	regexCheckBox.OnChanged = func(b bool) {
		if b {
			searchTextInput.Validate()
		}
		updateSearch()
	}
	searchTextInput.OnChanged = func(s string) {
		updateSearch()
	}

	content := container.NewBorder(
		container.NewBorder(
			container.NewBorder(
				nil,
				nil,
				separateFileAndFolders,
				container.NewHBox(
					regexCheckBox,
					flattenCheckBox,
				),
				searchTextInput,
			),
			nil,
			nil,
			container.New(
				columnLayout,
				headerButtons[1], // files
				headerButtons[2], // directories
				headerButtons[3], // size
				headerButtons[4], // percent
				headerButtons[5], // mod time
			),
			headerButtons[0], // name
		),
		nil,
		nil,
		nil,
		tree,
	)

	clean := func() {
		m.Clear()
		searchM.Clear()

		scheduleRefresh()
	}
	scan := util.SingleExecutionContextStopCtx(func(ctx context.Context) {
		startTime := time.Now()

		m.Clear()

		rootEntry := &Entry{
			IsDir: true,
		}
		m.Store("", rootEntry)

		for i, root := range roots {
			// the root path gets called without clean, all other paths are cleaned by walk
			root = filepath.Clean(root)
			roots[i] = root

			_, err := filewalkfast.WalkDataUnorderedMemoryEfficient(root, func(path string, d fs.FileInfo, err error, parent *Entry) (*Entry, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}

				if shouldSkip(path) {
					if d != nil && d.IsDir() {
						return nil, filepath.SkipDir
					}
					return nil, nil
				}

				e, alreadyExisted := m.Load(path)

				if !alreadyExisted || (e.Error != nil && err == nil) {
					e = &Entry{
						Name:  path,
						Error: err,
					}
					if d != nil {
						e.ModeTime = d.ModTime()
						e.IsDir = d.IsDir()
						if !e.IsDir {
							e.Size = datasize.ByteSize(d.Size())
						}
					}
					m.Store(path, e)
				}

				e.Parent = parent
				if e.IsDir {
					parent.Folders = append(parent.Folders, e)
				} else {
					parent.Files = append(parent.Files, e)
				}

				for p := parent; p != nil; p = p.Parent {
					p.Size += e.Size

					if p.ModeTime.Before(e.ModeTime) {
						p.ModeTime = e.ModeTime
					}

					p.FileCount += e.FileCount
					p.FolderCount += e.FolderCount

					if e.IsDir {
						p.FolderCount++
					} else {
						p.FileCount++
					}
				}

				updateSearchIfActive()

				if e.IsDir {
					if alreadyExisted {
						return e, filepath.SkipDir
					}
				}

				return e, nil
			}, rootEntry)
			if err != nil {
				log.Printf("err walk %s", err)
			}
		}

		updateSearchIfActive()

		duration := time.Since(startTime)
		log.Printf("file tree scan took %s", duration.String())
	})

	scheduleRefresh = Debounce(func() {
		myApp.Driver().DoFromGoroutine(func() {
			tree.Refresh()

			if columnLayout.NeedRerender {
				columnLayout.NeedRerender = false
				content.Refresh()
			}
		}, true)
	}, time.Second/60)

	return &FileTree{
		content: content,
		clean:   clean,
		scan:    scan,
	}
}

type Entry struct {
	Name        string
	Size        datasize.ByteSize
	ModeTime    time.Time
	IsDir       bool
	Error       error
	Parent      *Entry
	FolderCount int
	FileCount   int
	Folders     []*Entry
	Files       []*Entry
}

func (e *Entry) Root() *Entry {
	r := e
	for {
		p := r.Parent
		if p == nil {
			return r
		}
		r = p
	}
}

type SortBy uint8

const (
	SortByUnknown SortBy = iota
	SortByName
	SortBySize
	SortByFileCount
	SortByFolderCount
	SortByModTime
)

func SortEntries(sortBy SortBy, entries []*Entry, sortAscending bool) {
	mul := -1
	if sortAscending {
		mul = 1
	}

	slices.SortFunc(entries, func(a, b *Entry) int {
		switch sortBy {
		case SortBySize:
			return cmp.Compare(a.Size, b.Size) * mul
		case SortByFileCount:
			return cmp.Compare(a.FileCount, b.FileCount) * mul
		case SortByFolderCount:
			return cmp.Compare(a.FolderCount, b.FolderCount) * mul
		case SortByModTime:
			return a.ModeTime.Compare(b.ModeTime) * mul
		case SortByName:
			return cmp.Compare(a.Name, b.Name) * mul
		default:
			return 0
		}
	})
}

type ColumnLayout struct {
	RightToLeft  bool
	NeedRerender bool
	headerWidths []float32
	columnWidths []float32
}

func (l *ColumnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if l.RightToLeft {
		x := float32(size.Width)
		for i := min(len(objects), len(l.columnWidths)) - 1; i >= 0; i-- {
			obj := objects[i]
			w := l.columnWidths[i]
			obj.Resize(fyne.NewSize(w, size.Height))
			x -= w
			obj.Move(fyne.NewPos(x, 0))
		}
	} else {
		x := float32(0)
		for i, obj := range objects {
			if i >= len(l.columnWidths) {
				break
			}
			w := l.columnWidths[i]
			obj.Resize(fyne.NewSize(w, size.Height))
			obj.Move(fyne.NewPos(x, 0))
			x += w
		}
	}
}

func (l *ColumnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width float32
	var height float32
	for i, obj := range objects {
		ms := obj.MinSize()
		if i < len(l.columnWidths) {
			width += max(ms.Width, l.columnWidths[i])
		} else {
			width += ms.Width
		}
		if ms.Height > height {
			height = ms.Height
		}
	}
	return fyne.NewSize(width, height)
}

func (l *ColumnLayout) SetHeaderWidth(idx int, size fyne.Size) {
	l.headerWidths = EnsureLen(l.headerWidths, idx+1)

	if l.headerWidths[idx] < size.Width {
		l.headerWidths[idx] = size.Width
	}

	l.SetContentWidth(idx, size)
}

func (l *ColumnLayout) SetContentWidth(idx int, size fyne.Size) {
	l.columnWidths = EnsureLen(l.columnWidths, idx+1)

	if l.columnWidths[idx] < size.Width {
		l.columnWidths[idx] = size.Width
		l.NeedRerender = true
	}
}

func UpdateContentValue[T comparable](l *ColumnLayout, idx int, updateFunc func(T), elem fyne.CanvasObject, currentValue T, newValue T) {
	if currentValue == newValue {
		return
	}

	updateFunc(newValue)

	if idx < 0 {
		return
	}
	l.SetContentWidth(idx, elem.MinSize())
}

func UpdateHeaderValue[T comparable](l *ColumnLayout, idx int, updateFunc func(T), elem fyne.CanvasObject, currentValue T, newValue T) {
	if currentValue == newValue {
		return
	}

	updateFunc(newValue)

	if idx < 0 {
		return
	}
	l.SetHeaderWidth(idx, elem.MinSize())
}

type RightClickableLabel struct {
	*widget.Label
	OnRightClick func(e *fyne.PointEvent)
}

func NewRightClickableLabel(text string, onRightClick func(e *fyne.PointEvent)) *RightClickableLabel {
	l := &RightClickableLabel{
		Label:        widget.NewLabel(text),
		OnRightClick: onRightClick,
	}
	l.ExtendBaseWidget(l)
	return l
}

func (l *RightClickableLabel) Tapped(_ *fyne.PointEvent) {}

func (l *RightClickableLabel) TappedSecondary(e *fyne.PointEvent) {
	if l.OnRightClick != nil {
		l.OnRightClick(e)
	}
}

type RightClickableContainer struct {
	widget.BaseWidget
	Content      fyne.CanvasObject
	OnLeftClick  func(e *fyne.PointEvent, c *RightClickableContainer)
	OnRightClick func(e *fyne.PointEvent, c *RightClickableContainer)
}

func NewRightClickableContainer(content fyne.CanvasObject, onRightClick func(e *fyne.PointEvent, c *RightClickableContainer)) *RightClickableContainer {
	c := &RightClickableContainer{
		Content:      content,
		OnRightClick: onRightClick,
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *RightClickableContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.Content)
}

func (c *RightClickableContainer) Tapped(e *fyne.PointEvent) {
	if c.OnLeftClick != nil {
		c.OnLeftClick(e, c)
	}
}

func (c *RightClickableContainer) TappedSecondary(e *fyne.PointEvent) {
	if c.OnRightClick != nil {
		c.OnRightClick(e, c)
	}
}

var _ fyne.Tappable = (*RightClickableContainer)(nil)
var _ fyne.SecondaryTappable = (*RightClickableContainer)(nil)

var activePopup *widget.PopUp

// HidePopup hides the active popup
//
// this function is expected to be called from the main fyne thread
// => activePopup race condition should not happen if used correctly
func HidePopup() {
	if activePopup != nil {
		activePopup.Hide()
		activePopup = nil
	}
}

// this function is expected to be called from the main fyne thread
// => activePopup race condition should not happen if used correctly
func RightClickOpenContextMenu(e *fyne.PointEvent, c *RightClickableContainer, actions ...fyne.CanvasObject) {
	// Close previous popup if still open
	HidePopup()

	myApp := fyne.CurrentApp()

	menu := container.NewVBox(actions...)

	canvas := myApp.Driver().CanvasForObject(c)
	popup := widget.NewPopUp(menu, canvas)
	popup.ShowAtPosition(e.AbsolutePosition)

	// set size => click inside popup uses size, which would be 0 is not resized
	popup.Resize(menu.Size())
	popup.Resize(menu.Size())

	activePopup = popup
}

// this function is expected to be called from the main fyne thread
// => activePopup race condition should not happen if used correctly
func RightClickOpenPathActionsContextMenu(path string, e *fyne.PointEvent, c *RightClickableContainer, extraActions ...fyne.CanvasObject) {
	actions := []fyne.CanvasObject{
		widget.NewLabel(fmt.Sprintf("Actions for %s", filepath.Base(path))),
		widget.NewSeparator(),
		NewNewLabelSelectable(path),
		widget.NewButton("Open in explorer", func() {
			open.Explorer(path)
			HidePopup()
		}),
		widget.NewButton("Copy full path", func() {
			fyne.CurrentApp().Clipboard().SetContent(path)
			HidePopup()
		}),
		widget.NewButton("Copy name", func() {
			fyne.CurrentApp().Clipboard().SetContent(filepath.Base(path))
			HidePopup()
		}),
	}

	if len(extraActions) > 0 {
		actions = append(actions, widget.NewSeparator())
		actions = append(actions, extraActions...)
	}

	RightClickOpenContextMenu(e, c, actions...)
}

// widget.Label with Selectable-flag set (convince function)
//
// Attention: Right click gets trapped, so do not us inside a RightClickableContainer
func NewNewLabelSelectable(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Selectable = true
	return l
}

// EnsureLen grows the slice to at least n elements.
// If the slice is already long enough, it is returned unchanged.
func EnsureLen[T any](s []T, n int) []T {
	if len(s) >= n {
		return s
	}
	return append(s, make([]T, n-len(s))...)
}

// ConcatSeq returns an iter.Seq[T] that lazily iterates over all provided slices
// in order, without allocating or copying. The returned sequence yields each
// element from each slice until either all values are exhausted or the yield
// function returns false.
func ConcatSeq[T any](values ...[]T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, valSlice := range values {
			for _, val := range valSlice {
				if !yield(val) {
					return
				}
			}
		}
	}
}
