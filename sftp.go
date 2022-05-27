package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func NewSftpClient(server, user, pass, dir string) (*SftpClient, error) {
	if !strings.HasSuffix(server, ":22") && !strings.Contains(server, ":") {
		server = server + ":22"
	}
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	sshClient, err := ssh.Dial("tcp", server, sshConfig)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}

	h, err := sftpClient.Getwd()
	if err != nil {
		return nil, err
	}

	return &SftpClient{
		Dir:    h,
		client: sftpClient,
	}, nil

}

type SftpClient struct {
	Dir    string
	client *sftp.Client
}

func (c *SftpClient) List(ctx context.Context, prefix, marker string) (data []File, nextMarker string, err error) {
	log.WithFields(log.Fields{
		"marker": marker,
		"prefix": prefix,
	}).Debug("s3 list")
	if prefix == "" {
		prefix = c.Dir
	}
	fis, err := c.client.ReadDir(prefix)
	if err != nil {
		err = fmt.Errorf("readDir %s error %w", prefix, err)
		return
	}

	data = make([]File, len(fis))
	for i, v := range fis {
		f := File{
			Name: v.Name(),
			Type: FileRegular,
			Size: v.Size(),
			Time: v.ModTime(),
		}
		if v.IsDir() {
			f.Type = FileDir
			f.Name += "/"
		}

		data[i] = f
	}

	return
}

func (c *SftpClient) Upload(ctx context.Context, rs io.ReadSeeker, key, contentType string) (err error) {
	f, err := c.client.Create(key)
	if err != nil {
		err = fmt.Errorf("create %s error %w", key, err)
		return
	}
	_, err = io.Copy(f, rs)
	return
}

func (c *SftpClient) Download(ctx context.Context, w io.Writer, key string) (err error) {
	f, err := c.client.Open(key)
	if err != nil {
		err = fmt.Errorf("open %s error %w", key, err)
		return err
	}
	_, err = io.Copy(w, f)
	return
}

func (c *SftpClient) Delete(ctx context.Context, key string) (err error) {
	return c.client.Remove(key)
}

func (c *SftpClient) Stat(ctx context.Context, key string) (f File, err error) {
	fi, err := c.client.Stat(key)
	if err != nil {
		err = fmt.Errorf("stat %s error %w", key, err)
		return
	}
	f.Name = fi.Name()
	if fi.IsDir() {
		f.Type = FileDir
	}
	f.Size = fi.Size()
	f.Time = fi.ModTime()
	return
}

func (c *SftpClient) Close(ctx context.Context) error {
	return c.client.Close()
}
