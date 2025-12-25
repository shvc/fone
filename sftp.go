package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func NewSftpClient(server, user, pass, dir string) (*SftpClient, string, error) {
	if !strings.HasSuffix(server, ":22") && !strings.Contains(server, ":") {
		server = server + ":22"
	}

	config := &ssh.ClientConfig{
		Timeout: 10 * time.Second,
		User:    user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				// Just send the password back for all questions
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = pass
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		//HostKeyCallback: ssh.FixedHostKey(hostKey),
	}

	sshClient, err := ssh.Dial("tcp", server, config)
	if err != nil {
		return nil, "", fmt.Errorf("dial %s error %w", server, err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, "", fmt.Errorf("sftp %s error %w", server, err)
	}
	if dir == "" {
		dir, err = sftpClient.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("getwd error %w", err)
		}
	}

	pwd, err := sftpClient.RealPath(dir)
	if err != nil {
		return nil, "", fmt.Errorf("realpath %s error %w", dir, err)
	}

	return &SftpClient{
		Pwd:    pwd,
		Client: sftpClient,
	}, pwd, nil

}

type SftpClient struct {
	Pwd string
	*sftp.Client
}

func (c *SftpClient) List(ctx context.Context, prefix, marker string) (data []File, nextMarker string, err error) {
	slog.Debug("sftp list",
		slog.String("marker", marker),
		slog.String("prefix", prefix),
	)
	if prefix == "" {
		prefix = c.Pwd
	}
	fis, err := c.ReadDir(prefix)
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
			if !strings.HasSuffix(f.Name, "/") {
				f.Name += "/"
			}
		}

		data[i] = f
	}

	return
}

func (c *SftpClient) Upload(ctx context.Context, rs io.ReadSeeker, key, contentType string) (err error) {
	f, err := c.Create(key)
	if err != nil {
		err = fmt.Errorf("create %s error %w", key, err)
		return
	}
	_, err = io.Copy(f, rs)
	return
}

func (c *SftpClient) Download(ctx context.Context, w io.Writer, key string) (err error) {
	f, err := c.Open(key)
	if err != nil {
		err = fmt.Errorf("open %s error %w", key, err)
		return err
	}
	_, err = io.Copy(w, f)
	return
}

func (c *SftpClient) Delete(ctx context.Context, key string) (err error) {
	return c.Remove(key)
}

func (c *SftpClient) Stat(ctx context.Context, key string) (f File, err error) {
	fi, err := c.Client.Stat(key)
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
	return c.Client.Close()
}
