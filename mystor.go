package main

import (
	"context"
	"errors"
	"io"

	log "github.com/sirupsen/logrus"
)

func NewMystorClient(server, user, pass, dir string) (*MystorClient, string, error) {
	return &MystorClient{
		Pwd: dir,
	}, dir, nil
}

type MystorClient struct {
	Pwd string
}

func (c *MystorClient) ListAllMyBuckets(ctx context.Context) (data []string, err error) {
	log.WithFields(log.Fields{}).Debug("myshare list buckets")

	err = errors.New("no ready")
	return
}

func (c *MystorClient) List(ctx context.Context, prefix, marker string) (data []File, nextMarker string, err error) {
	log.WithFields(log.Fields{
		"marker": marker,
		"prefix": prefix,
	}).Debug("mystor list")

	err = errors.New("no ready")
	return
}

func (c *MystorClient) Upload(ctx context.Context, rs io.ReadSeeker, key, contentType string) (err error) {
	err = errors.New("no ready")
	return
}

func (c *MystorClient) Download(ctx context.Context, w io.Writer, key string) (err error) {
	err = errors.New("no ready")
	return
}

func (c *MystorClient) Delete(ctx context.Context, key string) (err error) {
	err = errors.New("no ready")
	return
}

func (c *MystorClient) Stat(ctx context.Context, key string) (f File, err error) {
	err = errors.New("no ready")
	return
}

func (c *MystorClient) Close(ctx context.Context) error {
	return nil
}
