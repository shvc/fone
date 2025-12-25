package main

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		accessKey string
		secretKey string
		region    string
		endpoint  string
	}{
		{
			name:      "valid credentials",
			accessKey: "testAccessKey",
			secretKey: "testSecretKey",
			region:    "us-east-1",
			endpoint:  "https://s3.amazonaws.com",
		},
		{
			name:      "anonymous access",
			accessKey: "",
			secretKey: "",
			region:    "us-east-1",
			endpoint:  "https://s3.amazonaws.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.accessKey, tt.secretKey, tt.region, tt.endpoint)
			if client == nil {
				t.Errorf("NewClient() = nil, want non-nil")
			}
			if client.Client == nil {
				t.Errorf("NewClient().Client = nil, want non-nil")
			}
		})
	}
}

func TestNewClientWithBucket(t *testing.T) {
	bucket := "test-bucket"
	prefix := "test-prefix/"
	accessKey := "testAccessKey"
	secretKey := "testSecretKey"
	region := "us-east-1"
	endpoint := "https://s3.amazonaws.com"

	client := NewClientWithBucket(bucket, prefix, accessKey, secretKey, region, endpoint)
	if client == nil {
		t.Errorf("NewClientWithBucket() = nil, want non-nil")
	}
	if client.Bucket != bucket {
		t.Errorf("NewClientWithBucket().Bucket = %v, want %v", client.Bucket, bucket)
	}
	if client.Prefix != prefix {
		t.Errorf("NewClientWithBucket().Prefix = %v, want %v", client.Prefix, prefix)
	}
}

func TestS3Client_Close(t *testing.T) {
	client := &S3Client{
		Bucket: "test-bucket",
	}

	err := client.Close(context.Background())
	if err != nil {
		t.Errorf("S3Client.Close() error = %v, want nil", err)
	}
}
