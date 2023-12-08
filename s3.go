package main

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	listDelimiter = "/"
)

var transport http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	Dial: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).Dial,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 0,
	MaxIdleConnsPerHost:   512,
	MaxIdleConns:          1024,
	IdleConnTimeout:       5 * time.Minute,
	TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true,
	},
}

func NewClientWithBucket(bucket, prefix, accessKey, secretKey, region, endpoint string) *S3Client {
	c := NewClient(accessKey, secretKey, region, endpoint)
	c.Bucket = bucket
	c.Prefix = prefix
	return c
}

func NewClient(accessKey, secretKey, region, endpoint string) *S3Client {
	awsConfig := aws.Config{
		Region:        region,
		ClientLogMode: 0,
		HTTPClient: &http.Client{
			Transport: transport,
		},
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, rg string, opts ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           endpoint,
				SigningName:   "s3",
				SigningRegion: rg,
			}, nil
		}),
		Retryer: func() aws.Retryer {
			return aws.NopRetryer{}
		},
	}

	if accessKey == "" && secretKey == "" {
		awsConfig.Credentials = aws.AnonymousCredentials{}
	} else {
		awsConfig.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     accessKey,
				SecretAccessKey: secretKey,
			}, nil
		})
	}

	client := s3.NewFromConfig(awsConfig, func(opts *s3.Options) {
		opts.UsePathStyle = true
	})

	return &S3Client{
		Client: client,
	}
}

type S3Client struct {
	Bucket string
	Prefix string
	*s3.Client
}

func (c *S3Client) ListAllMyBuckets(ctx context.Context) (data []string, err error) {
	slog.Debug("s3 list buckets")

	s3out, err := c.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return
	}

	data = make([]string, len(s3out.Buckets))
	for i, v := range s3out.Buckets {
		data[i] = *v.Name
	}

	return
}

func (c *S3Client) List(ctx context.Context, prefix, marker string) (data []File, nextMarker string, err error) {
	slog.Debug("s3 list",
		slog.String("marker", marker),
		slog.String("prefix", prefix),
	)
	loi := &s3.ListObjectsInput{
		Bucket:    aws.String(c.Bucket),
		Delimiter: aws.String(listDelimiter),
	}
	prefix = c.Prefix + prefix
	if prefix != "" {
		loi.Prefix = aws.String(prefix)
	}
	if marker != "" {
		loi.Marker = aws.String(marker)
	}
	if prefix != "" {
		loi.Prefix = aws.String(prefix)
	}

	s3out, err := c.ListObjects(ctx, loi)
	if err != nil {
		return
	}

	lp := len(s3out.CommonPrefixes)
	lc := len(s3out.Contents)
	data = make([]File, lp+lc)
	for i, v := range s3out.CommonPrefixes {
		f := File{
			Name: *v.Prefix,
			Type: FileDir,
		}
		if prefix != "" {
			f.Name = strings.TrimPrefix(f.Name, prefix)
		}
		data[i] = f
	}
	for i, v := range s3out.Contents {
		f := File{
			Name: *v.Key,
			Time: *v.LastModified,
			Size: *v.Size,
		}
		if prefix != "" {
			f.Name = strings.TrimPrefix(f.Name, prefix)
		}
		data[lp+i] = f
	}

	if *s3out.IsTruncated {
		if s3out.NextMarker != nil && *s3out.NextMarker != "" {
			nextMarker = *s3out.NextMarker
		} else {
			nextMarker = prefix + data[lp+lc-1].Name
		}
	}

	return
}

func (c *S3Client) Upload(ctx context.Context, rs io.ReadSeeker, key, contentType string) (err error) {
	if c.Prefix != "" {
		key = c.Prefix + key
	}
	input := &s3.PutObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
		Body:   rs,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err = c.PutObject(ctx, input, s3.WithAPIOptions(
		v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware,
	))

	return
}

func (c *S3Client) Download(ctx context.Context, w io.Writer, key string) (err error) {
	if c.Prefix != "" {
		key = c.Prefix + key
	}
	resp, err := c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)

	return
}

func (c *S3Client) Delete(ctx context.Context, key string) (err error) {
	if c.Prefix != "" {
		key = c.Prefix + key
	}
	_, err = c.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})

	return
}

func (c *S3Client) Stat(ctx context.Context, key string) (f File, err error) {
	if c.Prefix != "" {
		key = c.Prefix + key
	}
	resp, err := c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return
	}
	f.Name = key
	f.Size = *resp.ContentLength
	f.ContentType = *resp.ContentType
	f.Time = *resp.LastModified

	return
}

func (c *S3Client) Close(ctx context.Context) error {
	return nil
}
