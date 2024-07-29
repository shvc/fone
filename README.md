# fone
fone is a simple S3/sftp Browser based on [fyne](https://github.com/fyne-io/fyne)

# Install
- Download prebuilt [binary](https://github.com/shvc/fone/releases)
- Or build from source
```
git clone https://github.com/shvc/fone

go install github.com/fyne-io/fyne-cross

fyne-cross windows -env GOPROXY=https://goproxy.cn
fyne-cross linux -release -env GOPROXY=https://goproxy.cn
fyne-cross android -release -env GOPROXY=https://goproxy.cn
```
