package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	log "github.com/sirupsen/logrus"
)

const (
	shvcFone = "https://github.com/shvc/fone"
)

type contextButtonMenu struct {
	widget.Button
	menu *fyne.Menu
}

func (b *contextButtonMenu) Tapped(e *fyne.PointEvent) {
	widget.ShowPopUpMenuAtPosition(b.menu, fyne.CurrentApp().Driver().CanvasForObject(b), e.AbsolutePosition)
}

func buttonMenu(icon fyne.Resource, menu *fyne.Menu) *contextButtonMenu {
	b := &contextButtonMenu{menu: menu}
	b.SetIcon(icon)
	b.Importance = widget.LowImportance
	b.ExtendBaseWidget(b)
	return b
}

type Fone struct {
	a             fyne.App
	w             fyne.Window
	header        *fyne.Container
	refreshLock   bool
	body          *FileList
	footer        *fyne.Container
	client        *Client
	selectItemID  int
	selectFile    File
	btnRefresh    *widget.Button
	refreshCtx    context.Context
	refreshCancel context.CancelFunc
	pathLabel     *widget.Label
	infoLabel     *widget.Label
	loginForm     *widget.Form
	bucketEntry   *widget.SelectEntry
}

func splitKeyValue(data, sep string) (string, string) {
	bo := strings.SplitN(data, sep, 2)
	if len(bo) == 2 {
		return bo[0], bo[1]
	}

	return data, ""
}

func showLableMsg(l *widget.Label, msg string) {
	// handle UTF8 ?
	if msgLen := len(msg); msgLen > 69 {
		msg = msg[:42] + " ... " + msg[msgLen-21:]
	}
	l.SetText(msg)
}

func (sc *Fone) lockRefresh() {
	if sc.btnRefresh != nil {
		sc.btnRefresh.SetIcon(theme.CancelIcon())
	}
	sc.refreshLock = true
}

func (sc *Fone) unlockRefresh() {
	if sc.btnRefresh != nil {
		sc.btnRefresh.SetIcon(theme.ViewRefreshIcon())
	}
	sc.refreshLock = false
}

func (sc *Fone) makeHeader() error {
	sc.btnRefresh = widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		if sc.refreshLock {
			d := dialog.NewConfirm("Cancel", "Cancel current Refreshing?", func(b bool) {
				if b && sc.refreshCancel != nil {
					sc.refreshCancel()
				}
			}, sc.w)
			d.Show()
			return
		}
		sc.refreshCtx, sc.refreshCancel = context.WithCancel(context.Background())
		sc.lockRefresh()
		go func() {
			prefix := sc.pathLabel.Text
			data, nextMarker, err := sc.client.List(sc.refreshCtx, prefix, "")
			if err != nil {
				log.WithFields(log.Fields{
					"marker": "",
					"prefix": prefix,
					"error":  err.Error(),
				}).Warn("refresh failed")
				sc.unlockRefresh()
				showLableMsg(sc.infoLabel, err.Error())
				return
			}
			sc.infoLabel.SetText("")
			sc.body.Update(prefix, data)
			sc.appendBody(sc.refreshCtx, prefix, nextMarker)
		}()

	})
	sc.btnRefresh.Importance = widget.LowImportance

	bucketItem := fyne.NewMenuItem("Bucket", nil)
	bucketItem.ChildMenu = fyne.NewMenu("",
		fyne.NewMenuItem("Policy", func() {
			log.Info("Bucket Policy")
		}),
		fyne.NewMenuItem("Versioning", func() {
			log.Info("Bucket Versioning")
		}),
	)
	link, err := url.Parse(shvcFone)
	if err != nil {
		log.Warn("Could not parse URL", err)
	}
	menuLabel := buttonMenu(theme.MenuIcon(), fyne.NewMenu("",
		bucketItem,
		fyne.NewMenuItem("About", func() {
			dialog.NewCustom("About", "OK", widget.NewHyperlink(shvcFone, link), sc.w).Show()
		}),
		fyne.NewMenuItem("Exit", func() {
			dialog.NewConfirm("Exit", "Exit current Bucket?", func(ok bool) {
				if ok {
					sc.w.SetContent(sc.loginForm)
					sc.w.Resize(fyne.NewSize(600, 300))
					log.WithFields(log.Fields{
						"bucket": sc.bucketEntry.Text,
					}).Info("exit bucket")
				}
			}, sc.w).Show()
		}),
	))

	rightWidgets := container.NewHBox(
		sc.btnRefresh,
		menuLabel,
	)

	btnBackward := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if sc.pathLabel.Text == "" {
			sc.body.ScrollTo(0)
			return
		}

		if sc.refreshLock {
			d := dialog.NewConfirm("Cancel", "Cancel current Refreshing?", func(b bool) {
				if b && sc.refreshCancel != nil {
					sc.refreshCancel()
				}
			}, sc.w)
			d.Show()
			return
		}

		sc.refreshCtx, sc.refreshCancel = context.WithCancel(context.Background())
		sc.lockRefresh()
		go func() {
			prefix := path.Dir(strings.TrimSuffix(sc.pathLabel.Text, "/"))
			if prefix == "." {
				prefix = ""
			} else {
				prefix = prefix + "/"
			}
			data, nextMarker, err := sc.client.List(sc.refreshCtx, prefix, "")
			if err != nil {
				log.WithFields(log.Fields{
					"marker": "",
					"prefix": prefix,
					"error":  err.Error(),
				}).Warn("refresh failed")
				sc.unlockRefresh()
				showLableMsg(sc.infoLabel, err.Error())
				return
			}
			sc.infoLabel.SetText("")
			sc.pathLabel.SetText(prefix)
			sc.body.Update(prefix, data)
			sc.appendBody(sc.refreshCtx, prefix, nextMarker)
		}()

	})
	btnBackward.Importance = widget.LowImportance

	sc.pathLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: false})
	leftWidgets := container.NewHBox(
		btnBackward,
		sc.pathLabel,
	)

	sc.header = container.NewBorder(
		nil,
		nil,
		leftWidgets,
		rightWidgets,
		rightWidgets, leftWidgets,
	)
	return nil
}

func (sc *Fone) makeFooter() error {
	var btnDownload *widget.Button
	var downloadCtx context.Context
	var downloadCancel context.CancelFunc
	btnDownload = widget.NewButtonWithIcon("", theme.DownloadIcon(), func() {
		if btnDownload.Icon.Name() != "download.svg" {
			d := dialog.NewConfirm("Cancel", "Cancel Downloading?", func(b bool) {
				if b && downloadCancel != nil {
					downloadCancel()
				}
			}, sc.w)
			d.Show()
			return
		}
		if sc.selectFile.Name == "" || sc.selectFile.IsDir() {
			sc.infoLabel.SetText("Warn: No file chosen to download!")
			return
		}
		d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, e error) {
			if e != nil {
				log.WithFields(log.Fields{
					"error": e.Error(),
				}).Warn("download select file failed")
				btnDownload.SetIcon(theme.DownloadIcon())
				return
			}
			if uc == nil {
				log.Warn("download select file nil")
				btnDownload.SetIcon(theme.DownloadIcon())
				return
			}
			if sc.selectFile.IsDir() {
				btnDownload.SetIcon(theme.DownloadIcon())
				return
			}
			downloadCtx, downloadCancel = context.WithCancel(context.Background())
			go func() {
				key := sc.pathLabel.Text + sc.selectFile.Name
				defer btnDownload.SetIcon(theme.DownloadIcon())
				defer uc.Close()

				err := sc.client.Download(downloadCtx, uc, key)
				if err != nil {
					log.WithFields(log.Fields{
						"key":   key,
						"error": err.Error(),
					}).Warn("download failed")
					showLableMsg(sc.infoLabel, err.Error())
					return
				}
				log.WithFields(log.Fields{
					"key":  key,
					"file": uc.URI().String(),
				}).Info("download success")
			}()
		}, sc.w)
		d.SetFileName(path.Base(sc.selectFile.Name))
		d.Show()
		btnDownload.SetIcon(theme.CancelIcon())
	})
	btnDownload.Importance = widget.LowImportance

	var btnUpload *widget.Button
	var uploadCtx context.Context
	var uploadCancel context.CancelFunc
	btnUpload = widget.NewButtonWithIcon("", theme.UploadIcon(), func() {
		if btnUpload.Icon.Name() != "upload.svg" {
			d := dialog.NewConfirm("Cancel", "Cancel Uploading?", func(b bool) {
				if b && downloadCancel != nil {
					uploadCancel()
				}
			}, sc.w)
			d.Show()
			return
		}
		d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, e error) {
			if e != nil {
				log.WithFields(log.Fields{
					"error": e.Error(),
				}).Warn("upload select file failed")
				dialog.NewError(e, sc.w).Show()
				btnUpload.SetIcon(theme.UploadIcon())
				return
			}
			if uc == nil {
				sc.infoLabel.SetText("Warn: No file chosen to upload!")
				dialog.NewError(errors.New("nil file"), sc.w).Show()
				btnUpload.SetIcon(theme.UploadIcon())
				return
			}

			uploadCtx, uploadCancel = context.WithCancel(context.Background())
			go func() {
				defer btnUpload.SetIcon(theme.UploadIcon())
				defer uc.Close()
				filename := uc.URI().Name()
				key := sc.pathLabel.Text + filename
				fullName := uc.URI().Path()

				rs, ok := uc.(io.ReadSeeker)
				if !ok {
					// https://github.com/fyne-io/fyne/issues/2779
					data, err := io.ReadAll(uc)
					if err != nil {
						dialog.NewError(err, sc.w).Show()
						return
					}
					rs = bytes.NewReader(data)
				}
				err := sc.client.Upload(uploadCtx, rs, key, uc.URI().MimeType())
				if err != nil {
					log.WithFields(log.Fields{
						"key":   key,
						"file":  fullName,
						"error": err.Error(),
					}).Warn("upload failed")
					dialog.NewError(err, sc.w).Show()
					return
				}

				log.WithFields(log.Fields{
					"key":  key,
					"file": fullName,
				}).Info("upload success")

				sc.body.Add(File{
					Name: filename,
					Size: 1,
					Time: time.Now(),
				})
			}()
		}, sc.w)
		d.Show()
		btnUpload.SetIcon(theme.CancelIcon())
	})
	btnUpload.Importance = widget.LowImportance

	btnDelete := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		if sc.selectFile.IsDir() || sc.selectFile.Name == "" {
			sc.infoLabel.SetText("Warn: No file chosen to delete!")
			return
		}
		key := path.Join(sc.pathLabel.Text, sc.selectFile.Name)
		e := sc.client.Delete(context.TODO(), key)
		if e != nil {
			log.WithFields(log.Fields{
				"key":   key,
				"error": e.Error(),
			}).Warn("delete failed")
			return
		}
		sc.body.Delete(sc.selectItemID)
		log.WithFields(log.Fields{
			"key": key,
		}).Info("delete success")
	})
	btnDelete.Importance = widget.LowImportance

	rightWidgets := container.NewHBox(
		btnUpload,
		btnDownload,
		btnDelete,
	)

	sc.infoLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: false})
	leftWidgets := container.NewHBox(
		sc.infoLabel,
	)

	sc.footer = container.NewBorder(
		nil,
		nil,
		leftWidgets,
		rightWidgets,
		rightWidgets, leftWidgets,
	)
	return nil
}

// Must lockRefresh
func (sc *Fone) appendBody(ctx context.Context, prefix, marker string) {
	log.WithFields(log.Fields{
		"prefix": prefix,
		"marker": marker,
	}).Debug("more list")
	go func() {
		defer sc.unlockRefresh()
		data := []File{}
		for marker != "" {
			d, m, err := sc.client.List(ctx, prefix, marker)
			if err != nil {
				log.WithFields(log.Fields{
					"prefix": prefix,
					"marker": marker,
					"err":    err.Error(),
				}).Error("more list failed")
				break
			}
			marker = m
			data = append(data, d...)
		}
		sc.body.Append(data)
	}()

}

func (sc *Fone) initBody(data []File) {
	sc.body = NewFileList(data, func(i int, parent string) {
		sc.selectItemID = i
		sc.selectFile = sc.body.SelectFile(i)
		if !sc.selectFile.IsDir() {
			showLableMsg(sc.infoLabel, sc.selectFile.Info())
			return
		}

		if sc.refreshLock {
			d := dialog.NewConfirm("Cancel", "Cancel current Listing?", func(b bool) {
				if b && sc.refreshCancel != nil {
					sc.refreshCancel()
				}
			}, sc.w)
			d.Show()
			return
		}
		sc.refreshCtx, sc.refreshCancel = context.WithCancel(context.Background())
		sc.lockRefresh()

		prefix := sc.pathLabel.Text + sc.selectFile.Name
		data, nextMarker, err := sc.client.List(sc.refreshCtx, prefix, "")
		if err != nil {
			sc.unlockRefresh()
			showLableMsg(sc.infoLabel, "Error:"+err.Error())
			return
		}
		sc.pathLabel.SetText(prefix)
		sc.body.Update(prefix, data)
		sc.appendBody(sc.refreshCtx, prefix, nextMarker)
	}, nil)
}

func main() {
	var logfile string
	var debug bool
	flag.StringVar(&logfile, "log", filepath.Join(os.TempDir(), "fone.log"), "log filename")
	flag.BoolVar(&debug, "debug", false, "debug log")
	flag.Parse()

	logfd, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	}
	log.SetOutput(logfd)

	sc := Fone{
		a: app.NewWithID("cc.shvc.fone"),
	}
	sc.w = sc.a.NewWindow("fone")

	endpoint := widget.NewEntryWithData(binding.BindPreferenceString("cred.host", sc.a.Preferences()))
	endpoint.SetPlaceHolder("http://192.168.0.8:9000")
	endpoint.Validator = validation.NewRegexp(`^(?:https?://)?(?:[^/.\s]+\.)*`, "not a valid endpoint address")
	region := widget.NewEntryWithData(binding.BindPreferenceString("cred.region", sc.a.Preferences()))
	region.SetPlaceHolder("cn-north-1")
	sc.bucketEntry = widget.NewSelectEntry(nil)
	sc.bucketEntry.Bind(binding.BindPreferenceString("cred.bucket", sc.a.Preferences()))
	user := widget.NewEntryWithData(binding.BindPreferenceString("cred.user", sc.a.Preferences()))
	pass := widget.NewPasswordEntry()
	pass.Bind(binding.BindPreferenceString("cred.pass", sc.a.Preferences()))

	sc.loginForm = &widget.Form{
		Items: []*widget.FormItem{
			widget.NewFormItem("Endpoint", endpoint),
			widget.NewFormItem("Region", region),
			widget.NewFormItem("AccessKey", user),
			widget.NewFormItem("SecretKey", pass),
			widget.NewFormItem("Bucket", sc.bucketEntry),
		},
		SubmitText: "Enter",
		OnSubmit: func() {
			if sc.bucketEntry.Text != "" {
				bucketName, keyPrefix := splitKeyValue(sc.bucketEntry.Text, "/")
				sc.w.SetTitle(bucketName)
				sc.client = NewClientWithBucket(bucketName, keyPrefix, user.Text, pass.Text, region.Text, endpoint.Text)
				sc.lockRefresh()
				data, nextMarker, err := sc.client.List(context.Background(), "", "")
				if err != nil {
					log.WithFields(log.Fields{
						"endpoint": endpoint.Text,
						"bucket":   sc.bucketEntry.Text,
						"user":     user.Text,
						"error":    err.Error(),
					}).Warn("list objects failed")
					var e error
					for err != nil {
						e = err
						err = errors.Unwrap(err)
					}
					dialog.ShowError(e, sc.w)
					return
				}
				log.WithFields(log.Fields{
					"endpoint": endpoint.Text,
					"bucket":   sc.bucketEntry.Text,
					"user":     user.Text,
				}).Info("list objects success")

				sc.makeHeader()
				sc.initBody(data)
				sc.makeFooter()

				sc.refreshCtx, sc.refreshCancel = context.WithCancel(context.Background())
				sc.lockRefresh()
				sc.appendBody(sc.refreshCtx, "", nextMarker)

				sc.w.SetContent(container.NewBorder(sc.header, sc.footer, nil, nil, sc.body))
				sc.w.Resize(fyne.NewSize(800, 600))
			} else {
				sc.client = NewClient(user.Text, pass.Text, region.Text, endpoint.Text)
				data, err := sc.client.ListBuckets(context.Background())
				if err != nil {
					log.WithFields(log.Fields{
						"endpoint": endpoint.Text,
						"user":     user.Text,
						"error":    err.Error(),
					}).Warn("list buckets failed")
					var e error
					for err != nil {
						e = err
						err = errors.Unwrap(err)
					}
					dialog.ShowError(e, sc.w)
					return
				}
				log.WithFields(log.Fields{
					"endpoint": endpoint.Text,
					"user":     user.Text,
				}).Info("list buckets success")
				if len(data) > 0 {
					sc.bucketEntry.SetOptions(data)
					sc.bucketEntry.SetText(data[0])
				}
			}
		},
	}

	sc.w.SetContent(sc.loginForm)
	sc.w.Resize(fyne.NewSize(600, 300))
	sc.w.CenterOnScreen()
	sc.w.ShowAndRun()
}
