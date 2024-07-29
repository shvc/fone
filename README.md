# fone
fone is a simple S3/sftp Browser based on [fyne](https://github.com/fyne-io/fyne)

# Install
- Download prebuilt [binary](https://github.com/shvc/fone/releases)
- Or build from source
```
# clone code
git clone https://github.com/shvc/fone

# install build toolkit
go install github.com/fyne-io/fyne-cross@latest

# build Windows binary
fyne-cross windows -env GOPROXY=https://goproxy.cn

# build Linux binary
fyne-cross linux -release -env GOPROXY=https://goproxy.cn

# build Android binary
fyne-cross android -release -env GOPROXY=https://goproxy.cn
```

