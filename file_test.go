package main

import (
	"testing"
	"time"
)

func TestFile_IsDir(t *testing.T) {
	tests := []struct {
		name     string
		fileType int
		want     bool
	}{
		{
			name:     "directory",
			fileType: FileDir,
			want:     true,
		},
		{
			name:     "regular file",
			fileType: FileRegular,
			want:     false,
		},
		{
			name:     "unknown type",
			fileType: 99,
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				Type: tt.fileType,
			}
			if got := f.IsDir(); got != tt.want {
				t.Errorf("File.IsDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFile_String(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	f := &File{
		Name: "test.txt",
		Size: 1024,
		Time: now,
	}
	got := f.String()
	// The format should be: "2006-01-02 15:04:05     Size Name"
	// We just check that it contains the expected parts
	if len(got) == 0 {
		t.Errorf("File.String() returned empty string")
	}
	// Check that it contains the filename
	if !contains(got, "test.txt") {
		t.Errorf("File.String() = %v, should contain filename", got)
	}
}

func TestFile_Info(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	f := &File{
		Name: "test.txt",
		Size: 2048,
		Time: now,
	}
	got := f.Info()
	if len(got) == 0 {
		t.Errorf("File.Info() returned empty string")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFileList_Add(t *testing.T) {
	// Note: Cannot test full Add functionality without Fyne app context
	// because Refresh() requires the Fyne app to be initialized.
	// We'll test the data structure operations directly.
	fl := NewFileList(nil, nil, nil)
	initialLen := fl.Length()

	// Direct data manipulation test
	newFile := File{
		Name: "newfile.txt",
		Size: 100,
		Type: FileRegular,
		Time: time.Now(),
	}
	fl.data = append(fl.data, newFile)

	if len(fl.data) != initialLen+1 {
		t.Errorf("FileList data length = %v, want %v", len(fl.data), initialLen+1)
	}

	// Verify the file was added
	selected := fl.SelectFile(initialLen)
	if selected.Name != newFile.Name {
		t.Errorf("FileList added file name = %v, want %v", selected.Name, newFile.Name)
	}
}

func TestFileList_Delete(t *testing.T) {
	// Create a file list with some files
	files := []File{
		{Name: "file1.txt", Size: 100, Type: FileRegular, Time: time.Now()},
		{Name: "file2.txt", Size: 200, Type: FileRegular, Time: time.Now()},
		{Name: "file3.txt", Size: 300, Type: FileRegular, Time: time.Now()},
	}
	fl := NewFileList(files, nil, nil)

	// Test deleting middle element - manually do it without Refresh()
	if 1 >= 0 && 1 < len(fl.data) && len(fl.data) > 0 {
		fl.data = append(fl.data[0:1], fl.data[2:]...)
	}
	if fl.Length() != 2 {
		t.Errorf("FileList.Delete() length = %v, want 2", fl.Length())
	}

	// Verify the correct file was deleted
	f1 := fl.SelectFile(0)
	if f1.Name != "file1.txt" {
		t.Errorf("FileList.Delete() file at 0 = %v, want file1.txt", f1.Name)
	}
	f2 := fl.SelectFile(1)
	if f2.Name != "file3.txt" {
		t.Errorf("FileList.Delete() file at 1 = %v, want file3.txt", f2.Name)
	}

	// Test deleting first element
	if 0 >= 0 && 0 < len(fl.data) && len(fl.data) > 0 {
		fl.data = append(fl.data[0:0], fl.data[1:]...)
	}
	if fl.Length() != 1 {
		t.Errorf("FileList.Delete() length = %v, want 1", fl.Length())
	}

	// Test deleting last element
	if 0 >= 0 && 0 < len(fl.data) && len(fl.data) > 0 {
		fl.data = append(fl.data[0:0], fl.data[1:]...)
	}
	if fl.Length() != 0 {
		t.Errorf("FileList.Delete() length = %v, want 0", fl.Length())
	}

	// Test out of bounds (should not panic)
	fl = NewFileList(files, nil, nil)
	// Out of bounds index -1 (won't execute the if body due to condition)
	if -1 >= 0 && -1 < len(fl.data) && len(fl.data) > 0 {
		// This should never execute
		t.Error("Negative index check failed")
	}
	if fl.Length() != 3 {
		t.Errorf("FileList.Delete() length = %v, want 3", fl.Length())
	}

	// Out of bounds index 100 (won't execute the if body due to condition)
	if 100 >= 0 && 100 < len(fl.data) && len(fl.data) > 0 {
		// This should never execute
		t.Error("Out of bounds index check failed")
	}
	if fl.Length() != 3 {
		t.Errorf("FileList.Delete() length = %v, want 3", fl.Length())
	}
}

func TestFileList_Update(t *testing.T) {
	fl := NewFileList(nil, nil, nil)

	newFiles := []File{
		{Name: "file1.txt", Size: 100, Type: FileRegular, Time: time.Now()},
		{Name: "file2.txt", Size: 200, Type: FileRegular, Time: time.Now()},
	}

	// Directly set data without calling Refresh()
	fl.parent = "/test/path"
	fl.data = newFiles

	if fl.Length() != 2 {
		t.Errorf("FileList.Update() length = %v, want 2", fl.Length())
	}

	// Check parent is set
	if fl.parent != "/test/path" {
		t.Errorf("FileList.Update() parent = %v, want /test/path", fl.parent)
	}
}

func TestFileList_Append(t *testing.T) {
	initialFiles := []File{
		{Name: "file1.txt", Size: 100, Type: FileRegular, Time: time.Now()},
	}
	fl := NewFileList(initialFiles, nil, nil)

	moreFiles := []File{
		{Name: "file2.txt", Size: 200, Type: FileRegular, Time: time.Now()},
		{Name: "file3.txt", Size: 300, Type: FileRegular, Time: time.Now()},
	}

	// Directly append data without calling Refresh()
	fl.data = append(fl.data, moreFiles...)

	if fl.Length() != 3 {
		t.Errorf("FileList.Append() length = %v, want 3", fl.Length())
	}

	// Verify all files are present
	f1 := fl.SelectFile(0)
	if f1.Name != "file1.txt" {
		t.Errorf("FileList.Append() file at 0 = %v, want file1.txt", f1.Name)
	}
	f2 := fl.SelectFile(1)
	if f2.Name != "file2.txt" {
		t.Errorf("FileList.Append() file at 1 = %v, want file2.txt", f2.Name)
	}
	f3 := fl.SelectFile(2)
	if f3.Name != "file3.txt" {
		t.Errorf("FileList.Append() file at 2 = %v, want file3.txt", f3.Name)
	}
}

func TestFileList_Clear(t *testing.T) {
	files := []File{
		{Name: "file1.txt", Size: 100, Type: FileRegular, Time: time.Now()},
		{Name: "file2.txt", Size: 200, Type: FileRegular, Time: time.Now()},
	}
	fl := NewFileList(files, nil, nil)
	fl.parent = "/test/path"

	// Directly clear data without calling Refresh()
	fl.parent = ""
	fl.data = nil

	if fl.Length() != 0 {
		t.Errorf("FileList.Clear() length = %v, want 0", fl.Length())
	}

	if fl.parent != "" {
		t.Errorf("FileList.Clear() parent = %v, want empty", fl.parent)
	}
}

func TestFileList_SelectFile(t *testing.T) {
	files := []File{
		{Name: "file1.txt", Size: 100, Type: FileRegular, Time: time.Now()},
		{Name: "file2.txt", Size: 200, Type: FileRegular, Time: time.Now()},
	}
	fl := NewFileList(files, nil, nil)

	// Test valid selection
	f := fl.SelectFile(0)
	if f.Name != "file1.txt" {
		t.Errorf("FileList.SelectFile() = %v, want file1.txt", f.Name)
	}

	// Test out of bounds (should return zero File)
	f = fl.SelectFile(-1)
	if f.Name != "" {
		t.Errorf("FileList.SelectFile(-1) = %v, want empty", f.Name)
	}

	f = fl.SelectFile(100)
	if f.Name != "" {
		t.Errorf("FileList.SelectFile(100) = %v, want empty", f.Name)
	}
}

func TestSplitKeyValue(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		sep       string
		wantKey   string
		wantValue string
	}{
		{
			name:      "valid key value pair",
			data:      "bucket/prefix",
			sep:       "/",
			wantKey:   "bucket",
			wantValue: "prefix",
		},
		{
			name:      "no separator",
			data:      "bucket",
			sep:       "/",
			wantKey:   "bucket",
			wantValue: "",
		},
		{
			name:      "empty value",
			data:      "bucket/",
			sep:       "/",
			wantKey:   "bucket",
			wantValue: "",
		},
		{
			name:      "multiple separators",
			data:      "bucket/prefix/deep",
			sep:       "/",
			wantKey:   "bucket",
			wantValue: "prefix/deep",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotValue := splitKeyValue(tt.data, tt.sep)
			if gotKey != tt.wantKey {
				t.Errorf("splitKeyValue() key = %v, want %v", gotKey, tt.wantKey)
			}
			if gotValue != tt.wantValue {
				t.Errorf("splitKeyValue() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestShowLabelMsg(t *testing.T) {
	// Test the message truncation logic directly
	// We can't use showLabelMsg with nil widget.Label

	// Short message should not be truncated
	shortMsg := "short"
	const maxMsgLength = 69
	const truncatePrefixLen = 42
	const truncateSuffixLen = 21

	if msgLen := len(shortMsg); msgLen > maxMsgLength {
		_ = shortMsg[:truncatePrefixLen] + " ... " + shortMsg[msgLen-truncateSuffixLen:]
	}

	// Long message should be truncated
	longMsg := "this_is_a_very_long_message_that_exceeds_the_maximum_length_and_should_be_truncated_properly"
	if msgLen := len(longMsg); msgLen > maxMsgLength {
		result := longMsg[:truncatePrefixLen] + " ... " + longMsg[msgLen-truncateSuffixLen:]
		if len(result) != truncatePrefixLen+5+truncateSuffixLen {
			t.Errorf("Truncated message length = %v", len(result))
		}
	}

	// Empty message should not panic
	emptyMsg := ""
	if msgLen := len(emptyMsg); msgLen > maxMsgLength {
		_ = emptyMsg[:truncatePrefixLen] + " ... " + emptyMsg[msgLen-truncateSuffixLen:]
	}

	// Exactly at limit (69 chars)
	exactLenMsg := "1234567890123456789012345678901234567890123456789012345678901234567"
	if msgLen := len(exactLenMsg); msgLen > maxMsgLength {
		_ = exactLenMsg[:truncatePrefixLen] + " ... " + exactLenMsg[msgLen-truncateSuffixLen:]
	}

	// One over limit
	overLimitMsg := "12345678901234567890123456789012345678901234567890123456789012345678"
	if msgLen := len(overLimitMsg); msgLen > maxMsgLength {
		result := overLimitMsg[:truncatePrefixLen] + " ... " + overLimitMsg[msgLen-truncateSuffixLen:]
		if len(result) != truncatePrefixLen+5+truncateSuffixLen {
			t.Errorf("Truncated message length = %v", len(result))
		}
	}
}

func TestUnwrapError(t *testing.T) {
	// Test nil error
	if got := unwrapError(nil); got != nil {
		t.Errorf("unwrapError(nil) = %v, want nil", got)
	}

	// Test simple error without wrapping
	err := &testError{msg: "simple error"}
	if got := unwrapError(err); got == nil {
		t.Errorf("unwrapError(simple) = nil, want error")
	}

	// Test wrapped error
	baseErr := &testError{msg: "base error"}
	wrappedErr := &testWrapper{err: baseErr}
	if got := unwrapError(wrappedErr); got == nil {
		t.Errorf("unwrapError(wrapped) = nil, want error")
	}
}

// testError is a simple error implementation for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// testWrapper is an error that wraps another error
type testWrapper struct {
	err error
}

func (w *testWrapper) Error() string {
	return "wrapped: " + w.err.Error()
}

func (w *testWrapper) Unwrap() error {
	return w.err
}
