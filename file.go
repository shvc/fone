package main

import (
	"fmt"
	"path"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	log "github.com/sirupsen/logrus"
)

const (
	FileRegular = 0
	FileDir     = 1
)

type File struct {
	Name        string
	Type        int
	Size        int64
	ContentType string
	Time        time.Time
}

func (f *File) String() string {
	return fmt.Sprintf("%s %8s %s",
		f.Time.Format("2006-01-02 15:04:05"),
		bytefmt.ByteSize(uint64(f.Size)),
		f.Name,
	)
}

func (f *File) Info() string {
	return fmt.Sprintf("%s %8s",
		f.Time.Format("2006-01-02 15:04:05"),
		bytefmt.ByteSize(uint64(f.Size)),
	)
}

func (f *File) IsDir() bool {
	return f.Type == FileDir
}

type FileList struct {
	parent string
	widget.List
	data []File
}

func NewFileList(vf []File, selectFn func(int, string), unSelectFn func()) *FileList {
	fl := &FileList{
		parent: "",
		data:   vf,
		List:   widget.List{},
	}
	fl.Length = func() int {
		return len(fl.data)
	}
	fl.CreateItem = func() fyne.CanvasObject {
		return container.NewHBox(widget.NewIcon(nil), widget.NewLabel(""))
	}

	fl.UpdateItem = func(id widget.ListItemID, item fyne.CanvasObject) {
		if id < 0 || id >= len(fl.data) {
			return
		}
		f := fl.data[id]
		if f.IsDir() {
			item.(*fyne.Container).Objects[0].(*widget.Icon).SetResource(theme.FolderIcon())
		} else {
			switch strings.ToLower(path.Ext(f.Name)) {
			case ".mp4":
				item.(*fyne.Container).Objects[0].(*widget.Icon).SetResource(theme.FileVideoIcon())
			case ".mp3":
				item.(*fyne.Container).Objects[0].(*widget.Icon).SetResource(theme.FileAudioIcon())
			case ".png", ".jpg", ".jpeg":
				item.(*fyne.Container).Objects[0].(*widget.Icon).SetResource(theme.FileImageIcon())
			case ".txt":
				item.(*fyne.Container).Objects[0].(*widget.Icon).SetResource(theme.FileTextIcon())
			default:
				item.(*fyne.Container).Objects[0].(*widget.Icon).SetResource(theme.FileIcon())
			}

		}
		item.(*fyne.Container).Objects[1].(*widget.Label).SetText(f.Name)
	}

	fl.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(fl.data) {
			return
		}

		if selectFn != nil {
			selectFn(id, fl.parent)
		}
	}

	fl.OnUnselected = func(id widget.ListItemID) {
		if unSelectFn != nil {
			unSelectFn()
		}
	}

	fl.ExtendBaseWidget(fl)
	return fl
}

func (fl *FileList) Add(f File) error {
	fl.data = append(fl.data, f)
	fl.Refresh()
	return nil
}

func (fl *FileList) Clear() {
	fl.parent = ""
	fl.data = nil
	fl.Refresh()
}

func (fl *FileList) Delete(id int) {
	fl.data = append(fl.data[0:id], fl.data[id+1:]...)
	fl.Refresh()
	fl.UnselectAll()
}

func (fl *FileList) Update(parent string, vv []File) error {
	fl.parent = parent
	fl.data = vv
	fl.Refresh()
	fl.UnselectAll()
	return nil
}

func (fl *FileList) Append(vv []File) error {
	fl.data = append(fl.data, vv...)
	fl.Refresh()
	fl.UnselectAll()
	return nil
}

func (fl *FileList) SelectFile(id int) (v File) {
	if id < 0 || id >= len(fl.data) {
		return
	}
	v = fl.data[id]

	log.WithFields(log.Fields{
		"id":     id,
		"name":   v.Name,
		"parent": fl.parent,
	}).Debug("select file")

	return
}
