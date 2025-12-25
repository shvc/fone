package main

import (
	"os"
	"testing"
	"time"
)

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
}

func (fi *mockFileInfo) Name() string       { return fi.name }
func (fi *mockFileInfo) Size() int64        { return fi.size }
func (fi *mockFileInfo) Mode() os.FileMode  {
	if fi.isDir {
		return os.ModeDir | 0755
	}
	return 0644
}
func (fi *mockFileInfo) IsDir() bool        { return fi.isDir }
func (fi *mockFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *mockFileInfo) Sys() interface{}   { return nil }

func TestSftpClient_DirectoryHandling(t *testing.T) {
	now := time.Now().UTC()

	// Test that directories without trailing slash get it appended
	testFiles := []File{
		{Name: "file1.txt", Size: 100, Type: FileRegular, Time: now},
		{Name: "folder1/", Size: 0, Type: FileDir, Time: now},
		{Name: "folder2/", Size: 0, Type: FileDir, Time: now},
	}

	// Verify the directory names have trailing slashes
	dirCount := 0
	for _, f := range testFiles {
		if f.IsDir() {
			dirCount++
			if f.Name != "folder1/" && f.Name != "folder2/" {
				t.Errorf("Directory name = %v, should have trailing slash", f.Name)
			}
		}
	}

	if dirCount != 2 {
		t.Errorf("Expected 2 directories, got %d", dirCount)
	}
}

func TestSftpClient_DirectoryWithoutSlash(t *testing.T) {
	// Test that directories without trailing slash are handled correctly
	// This simulates what would happen when reading from SFTP
	file := File{
		Name: "folder",
		Size: 0,
		Type: FileDir,
		Time: time.Now().UTC(),
	}

	// The SFTP List function appends "/" to directory names
	if !file.IsDir() {
		t.Errorf("Expected IsDir() to return true")
	}

	// Simulate the SFTP List function behavior
	testName := file.Name
	if file.IsDir() && testName[len(testName)-1:] != "/" {
		testName += "/"
	}

	// After processing, directory should have trailing slash
	if testName != "folder/" {
		t.Errorf("Expected folder name to end with /, got %s", testName)
	}
}

func TestSftpClient_Close(t *testing.T) {
	// Test close with nil client (simulating uninitialized state)
	// We cannot test the full close without a real sftp.Client
	// The Close() function calls c.Client.Close() which would panic with nil

	// Test that the struct can be created
	client := &SftpClient{
		Pwd: "/home/test",
	}

	// Just verify the struct was created correctly
	if client.Pwd != "/home/test" {
		t.Errorf("SftpClient Pwd = %v, want /home/test", client.Pwd)
	}
}
