package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
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

type provider interface {
	List(ctx context.Context, prefix, marker string) (data []File, nextMarker string, err error)
	Upload(ctx context.Context, rs io.ReadSeeker, key, contentType string) (err error)
	Download(ctx context.Context, w io.Writer, key string) (err error)
	Delete(ctx context.Context, key string) (err error)
	Stat(ctx context.Context, key string) (f File, err error)
	Close(ctx context.Context) error
}

type Fone struct {
	a             fyne.App
	w             fyne.Window
	header        *fyne.Container
	refreshLock   bool
	body          *FileList
	footer        *fyne.Container
	client        provider
	selectItemID  int
	selectFile    File
	btnRefresh    *widget.Button
	refreshCtx    context.Context
	refreshCancel context.CancelFunc
	pathLabel     *widget.Label
	infoLabel     *widget.Label
	appTab        *container.AppTabs
}

func splitKeyValue(data, sep string) (string, string) {
	bo := strings.SplitN(data, sep, 2)
	if len(bo) == 2 {
		return bo[0], bo[1]
	}

	return data, ""
}

func showLabelMsg(l *widget.Label, msg string) {
	const (
		maxMsgLength      = 69
		truncatePrefixLen = 42
		truncateSuffixLen = 21
	)
	if msgLen := len(msg); msgLen > maxMsgLength {
		msg = msg[:truncatePrefixLen] + " ... " + msg[msgLen-truncateSuffixLen:]
	}
	l.SetText(msg)
}

// unwrapError returns the root cause of an error by unwrapping it
func unwrapError(err error) error {
	for err != nil {
		nextErr := errors.Unwrap(err)
		if nextErr == nil {
			return err
		}
		err = nextErr
	}
	return nil
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
				slog.Warn("refresh failed",
					slog.String("marker", ""),
					slog.String("prefix", prefix),
					slog.String("error", err.Error()),
				)
				sc.unlockRefresh()
				showLabelMsg(sc.infoLabel, err.Error())
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
			slog.Info("Bucket Policy")
		}),
		fyne.NewMenuItem("Versioning", func() {
			slog.Info("Bucket Versioning")
		}),
	)
	link, err := url.Parse(shvcFone)
	if err != nil {
		slog.Warn("Could not parse URL",
			slog.String("url", shvcFone),
			slog.String("error", err.Error()),
		)
	}
	menuLabel := buttonMenu(theme.MenuIcon(), fyne.NewMenu("",
		bucketItem,
		fyne.NewMenuItem("About", func() {
			dialog.NewCustom("About", "OK", widget.NewHyperlink(shvcFone, link), sc.w).Show()
		}),
		fyne.NewMenuItem("Exit", func() {
			dialog.NewConfirm("Exit", "Exit current Session?", func(ok bool) {
				if ok {
					sc.w.SetContent(sc.appTab)
					sc.w.Resize(fyne.NewSize(600, 300))
					sc.selectFile.Name = ""
					sc.selectItemID = -1
					slog.Info("exit session",
						slog.String("pwd", sc.pathLabel.Text),
					)
				}
			}, sc.w).Show()
		}),
	))

	rightWidgets := container.NewHBox(
		sc.btnRefresh,
		menuLabel,
	)

	btnBackward := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if sc.pathLabel.Text == "" || sc.pathLabel.Text == "/" {
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
				if !strings.HasSuffix(prefix, "/") {
					prefix = prefix + "/"
				}
			}
			data, nextMarker, err := sc.client.List(sc.refreshCtx, prefix, "")
			if err != nil {
				slog.Warn("refresh failed",
					slog.String("marker", ""),
					slog.String("prefix", prefix),
					slog.String("error", err.Error()),
				)
				sc.unlockRefresh()
				showLabelMsg(sc.infoLabel, err.Error())
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
		if btnDownload.Icon.Name() != "foreground_download.svg" {
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
				slog.Warn("download select file failed",
					slog.String("error", e.Error()),
				)
				btnDownload.SetIcon(theme.DownloadIcon())
				return
			}
			if uc == nil {
				slog.Warn("download select file nil")
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
					slog.Warn("download failed",
						slog.String("key", key),
						slog.String("error", err.Error()),
					)
					showLabelMsg(sc.infoLabel, err.Error())
					return
				}

				slog.Info("download success",
					slog.String("key", key),
					slog.String("file", uc.URI().String()),
				)
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
		if btnUpload.Icon.Name() != "foreground_upload.svg" {
			d := dialog.NewConfirm("Cancel", "Cancel Uploading?", func(b bool) {
				if b && uploadCancel != nil {
					uploadCancel()
				}
			}, sc.w)
			d.Show()
			return
		}
		d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, e error) {
			if e != nil {
				slog.Warn("upload select file failed",
					slog.String("error", e.Error()),
				)
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
					slog.Warn("upload failed",
						slog.String("key", key),
						slog.String("file", fullName),
						slog.String("error", err.Error()),
					)
					dialog.NewError(err, sc.w).Show()
					return
				}

				slog.Info("upload success",
					slog.String("key", key),
					slog.String("file", fullName),
				)

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
			slog.Warn("delete failed",
				slog.String("key", key),
				slog.String("error", e.Error()),
			)
			return
		}
		sc.body.Delete(sc.selectItemID)
		slog.Info("delete success",
			slog.String("key", key),
		)
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
	slog.Debug("more list",
		slog.String("prefix", prefix),
		slog.String("marker", marker),
	)
	go func() {
		defer sc.unlockRefresh()
		data := []File{}
		for marker != "" {
			d, m, err := sc.client.List(ctx, prefix, marker)
			if err != nil {
				slog.Error("more list failed",
					slog.String("prefix", prefix),
					slog.String("marker", marker),
					slog.String("error", err.Error()),
				)
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
			showLabelMsg(sc.infoLabel, sc.selectFile.Info())
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
			showLabelMsg(sc.infoLabel, "Error:"+err.Error())
			return
		}
		sc.pathLabel.SetText(prefix)
		sc.body.Update(prefix, data)
		sc.appendBody(sc.refreshCtx, prefix, nextMarker)
	}, nil)
}

func (sc *Fone) createS3LoginForm() *widget.Form {
	endpoint := widget.NewEntryWithData(binding.BindPreferenceString("cred.s3_endpoint", sc.a.Preferences()))
	endpoint.SetPlaceHolder("http://192.168.0.8:9000")
	endpoint.Validator = validation.NewRegexp(`^(?:https?://)?(?:[^/.\s]+\.)*`, "not a valid endpoint address")
	region := widget.NewEntryWithData(binding.BindPreferenceString("cred.s3_region", sc.a.Preferences()))
	region.SetPlaceHolder("cn-north-1")
	bucketEntry := widget.NewSelectEntry(nil)
	bucketEntry.Bind(binding.BindPreferenceString("cred.s3_bucket", sc.a.Preferences()))
	user := widget.NewEntryWithData(binding.BindPreferenceString("cred.s3_user", sc.a.Preferences()))
	pass := widget.NewPasswordEntry()
	pass.Bind(binding.BindPreferenceString("cred.s3_pass", sc.a.Preferences()))

	return &widget.Form{
		Items: []*widget.FormItem{
			widget.NewFormItem("Endpoint", endpoint),
			widget.NewFormItem("Region", region),
			widget.NewFormItem("AccessKey", user),
			widget.NewFormItem("SecretKey", pass),
			widget.NewFormItem("Bucket", bucketEntry),
		},
		SubmitText: "Enter",
		OnSubmit: func() {
			sc.w.SetTitle("S3")
			if bucketEntry.Text != "" {
				bucketName, keyPrefix := splitKeyValue(bucketEntry.Text, "/")
				sc.client = NewClientWithBucket(bucketName, keyPrefix, user.Text, pass.Text, region.Text, endpoint.Text)
				sc.lockRefresh()
				data, nextMarker, err := sc.client.List(context.Background(), "", "")
				if err != nil {
					slog.Warn("list file failed",
						slog.String("endpoint", endpoint.Text),
						slog.String("bucket", bucketEntry.Text),
						slog.String("user", user.Text),
						slog.String("error", err.Error()),
					)
					dialog.ShowError(unwrapError(err), sc.w)
					return
				}
				slog.Info("list file success",
					slog.String("endpoint", endpoint.Text),
					slog.String("bucket", bucketEntry.Text),
					slog.String("user", user.Text),
				)

				sc.makeHeader()
				sc.initBody(data)
				sc.makeFooter()

				sc.refreshCtx, sc.refreshCancel = context.WithCancel(context.Background())
				sc.lockRefresh()
				sc.appendBody(sc.refreshCtx, "", nextMarker)

				sc.w.SetContent(container.NewBorder(sc.header, sc.footer, nil, nil, sc.body))
				sc.w.Resize(fyne.NewSize(800, 600))
			} else {
				client := NewClient(user.Text, pass.Text, region.Text, endpoint.Text)
				data, err := client.ListAllMyBuckets(context.Background())
				if err != nil {
					slog.Warn("list buckets failed",
						slog.String("endpoint", endpoint.Text),
						slog.String("user", user.Text),
						slog.String("error", err.Error()),
					)
					dialog.ShowError(unwrapError(err), sc.w)
					return
				}

				slog.Info("list buckets success",
					slog.String("endpoint", endpoint.Text),
					slog.String("user", user.Text),
				)
				if len(data) > 0 {
					bucketEntry.SetOptions(data)
					bucketEntry.SetText(data[0])
				}
			}
		},
	}
}

func (sc *Fone) createSftpLoginForm() *widget.Form {
	server := widget.NewEntryWithData(binding.BindPreferenceString("cred.sftp_server", sc.a.Preferences()))
	server.SetPlaceHolder("192.168.0.8:22")
	remoteDir := widget.NewEntryWithData(binding.BindPreferenceString("cred.sftp_dir", sc.a.Preferences()))
	sftpUser := widget.NewEntryWithData(binding.BindPreferenceString("cred.sftp_user", sc.a.Preferences()))
	sftpPassword := widget.NewPasswordEntry()
	sftpPassword.Bind(binding.BindPreferenceString("cred.sftp_password", sc.a.Preferences()))

	return &widget.Form{
		Items: []*widget.FormItem{
			widget.NewFormItem("Server", server),
			widget.NewFormItem("Directory", remoteDir),
			widget.NewFormItem("User", sftpUser),
			widget.NewFormItem("Password", sftpPassword),
		},
		SubmitText: "Enter",
		OnSubmit: func() {
			var err error
			var pwd string
			sc.w.SetTitle("sftp")
			sc.client, pwd, err = NewSftpClient(server.Text, sftpUser.Text, sftpPassword.Text, remoteDir.Text)
			if err != nil {
				slog.Warn("init provider failed",
					slog.String("server", server.Text),
					slog.String("pwd", pwd),
					slog.String("user", sftpUser.Text),
					slog.String("error", err.Error()),
				)
				return
			}

			sc.lockRefresh()
			data, nextMarker, err := sc.client.List(context.Background(), pwd, "")
			if err != nil {
				slog.Warn("list file failed",
					slog.String("server", server.Text),
					slog.String("pwd", pwd),
					slog.String("user", sftpUser.Text),
					slog.String("error", err.Error()),
				)
				dialog.ShowError(unwrapError(err), sc.w)
				return
			}

			slog.Info("list file success",
				slog.String("server", server.Text),
				slog.String("pwd", pwd),
				slog.String("user", sftpUser.Text),
			)

			sc.makeHeader()
			sc.initBody(data)
			sc.makeFooter()
			if !strings.HasSuffix(pwd, "/") {
				pwd += "/"
			}
			sc.pathLabel.SetText(pwd)

			sc.refreshCtx, sc.refreshCancel = context.WithCancel(context.Background())
			sc.lockRefresh()
			sc.appendBody(sc.refreshCtx, "", nextMarker)

			sc.w.SetContent(container.NewBorder(sc.header, sc.footer, nil, nil, sc.body))
			sc.w.Resize(fyne.NewSize(800, 600))
		},
	}
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
	logOpt := &slog.HandlerOptions{}
	if debug {
		logOpt.Level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logfd, logOpt)))

	fe := Fone{
		a: app.NewWithID("cc.shvc.fone"),
	}
	fe.w = fe.a.NewWindow("fone")

	fe.appTab = container.NewAppTabs(
		container.NewTabItemWithIcon("S3", theme.FileIcon(), fe.createS3LoginForm()),
		container.NewTabItemWithIcon("sftp", theme.FolderIcon(), fe.createSftpLoginForm()),
	)

	fe.w.SetContent(fe.appTab)
	fe.w.Resize(fyne.NewSize(600, 300))
	fe.w.CenterOnScreen()
	fe.w.ShowAndRun()
}
