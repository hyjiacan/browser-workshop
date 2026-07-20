package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownload_Basic(t *testing.T) {
	// Create test data
	testData := make([]byte, 100*1024) // 100KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		w.Write(testData)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile.bin")

	mgr := NewManager()

	var progressCount int32
	result, err := mgr.Download(context.Background(), Options{
		URL:      server.URL,
		DestPath: destPath,
		OnProgress: func(p Progress) {
			atomic.AddInt32(&progressCount, 1)
		},
		ProgressInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify file exists and has correct size
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat downloaded file: %v", err)
	}
	if info.Size() != int64(len(testData)) {
		t.Errorf("downloaded file size = %d, want %d", info.Size(), len(testData))
	}

	// Verify content
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if len(data) != len(testData) {
		t.Errorf("content length = %d, want %d", len(data), len(testData))
	}
	for i := range testData {
		if data[i] != testData[i] {
			t.Errorf("content mismatch at byte %d: got %d, want %d", i, data[i], testData[i])
			break
		}
	}

	// Verify result
	if result.Size != int64(len(testData)) {
		t.Errorf("result.Size = %d, want %d", result.Size, len(testData))
	}
	if result.Resumed {
		t.Error("result.Resumed should be false for fresh download")
	}
	if result.Path != destPath {
		t.Errorf("result.Path = %s, want %s", result.Path, destPath)
	}

	// Progress should have been reported
	if atomic.LoadInt32(&progressCount) == 0 {
		t.Error("no progress callbacks were made")
	}
}

func TestDownload_Resume(t *testing.T) {
	// Create test data (50KB)
	testData := make([]byte, 50*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Track requests to verify range header
	var rangeRequest bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			rangeRequest = true
			// Parse range: bytes=START-
			var start int
			fmt.Sscanf(rangeHeader, "bytes=%d-", &start)

			// Return partial content
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(testData)-1, len(testData)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)-start))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(testData[start:])
			return
		}

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		w.Write(testData)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "testfile.bin")
	tempPath := destPath + ".part"

	// Create a partial file (first 20KB)
	partialData := testData[:20*1024]
	if err := os.WriteFile(tempPath, partialData, 0o644); err != nil {
		t.Fatalf("creating partial file: %v", err)
	}

	mgr := NewManager()

	result, err := mgr.Download(context.Background(), Options{
		URL:      server.URL,
		DestPath: destPath,
		Resume:   true,
	})
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Should have made a range request
	if !rangeRequest {
		t.Error("expected Range request for resume, but none was made")
	}

	// Should have been resumed
	if !result.Resumed {
		t.Error("result.Resumed should be true for resumed download")
	}

	// Verify complete file
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if len(data) != len(testData) {
		t.Errorf("downloaded size = %d, want %d", len(data), len(testData))
	}
	for i := range testData {
		if data[i] != testData[i] {
			t.Errorf("content mismatch at byte %d", i)
			break
		}
	}

	// .part file should be gone
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error(".part file should have been removed")
	}
}

func TestDownload_Cancel(t *testing.T) {
	// Slow server that sends data slowly
	testData := make([]byte, 1024*1024) // 1MB

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		// Write in small chunks with delays
		chunkSize := 1024
		for i := 0; i < len(testData); i += chunkSize {
			end := i + chunkSize
			if end > len(testData) {
				end = len(testData)
			}
			w.Write(testData[i:end])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "bigfile.bin")

	mgr := NewManager()

	// Start download in goroutine
	errCh := make(chan error, 1)
	go func() {
		_, err := mgr.Download(context.Background(), Options{
			URL:      server.URL,
			DestPath: destPath,
		})
		errCh <- err
	}()

	// Wait a bit then cancel
	time.Sleep(50 * time.Millisecond)
	cancelled := mgr.Cancel(server.URL)
	if !cancelled {
		t.Error("Cancel returned false, expected true")
	}

	// Wait for download to finish
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected error after cancellation, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("download did not finish after cancellation")
	}
}

func TestDownload_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testData := make([]byte, 1024*1024)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		// Slow write
		for i := 0; i < len(testData); i += 1024 {
			w.Write(testData[i : i+1024])
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "test.bin")

	mgr := NewManager()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := mgr.Download(ctx, Options{
			URL:      server.URL,
			DestPath: destPath,
		})
		errCh <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected context error, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("download did not stop after context cancellation")
	}
}

func TestDownload_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "test.bin")

	mgr := NewManager()

	_, err := mgr.Download(context.Background(), Options{
		URL:      server.URL,
		DestPath: destPath,
	})
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 512, "512.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		bps  float64
		want string
	}{
		{500, "500 B/s"},
		{1500, "1.5 KB/s"},
		{1024 * 1024, "1.0 MB/s"},
	}

	for _, tt := range tests {
		got := FormatSpeed(tt.bps)
		if got != tt.want {
			t.Errorf("FormatSpeed(%f) = %q, want %q", tt.bps, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{2 * time.Hour, "2h 0m"},
	}

	for _, tt := range tests {
		got := FormatDuration(tt.d)
		if got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestDownload_NoResumeOnServerNotSupporting(t *testing.T) {
	testData := make([]byte, 50*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Server that ignores Range header and always returns full content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		w.WriteHeader(http.StatusOK) // Not 206 Partial Content
		w.Write(testData)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "test.bin")
	tempPath := destPath + ".part"

	// Create garbage partial file
	if err := os.WriteFile(tempPath, []byte("garbage data here"), 0o644); err != nil {
		t.Fatalf("creating partial file: %v", err)
	}

	mgr := NewManager()

	result, err := mgr.Download(context.Background(), Options{
		URL:      server.URL,
		DestPath: destPath,
		Resume:   true,
	})
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Should not have been resumed (server doesn't support it)
	if result.Resumed {
		t.Error("result.Resumed should be false when server doesn't support range")
	}

	// File should be complete and correct
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) != len(testData) {
		t.Errorf("file size = %d, want %d", len(data), len(testData))
	}
	for i := range testData {
		if data[i] != testData[i] {
			t.Errorf("content mismatch at byte %d", i)
			break
		}
	}
}

func TestDownload_Validation(t *testing.T) {
	mgr := NewManager()

	// Missing URL
	_, err := mgr.Download(context.Background(), Options{
		DestPath: "/tmp/test",
	})
	if err == nil {
		t.Error("expected error for missing URL")
	}

	// Missing dest path
	_, err = mgr.Download(context.Background(), Options{
		URL: "http://example.com",
	})
	if err == nil {
		t.Error("expected error for missing dest path")
	}
}

// Ensure the progressWriter correctly handles context cancellation
func TestProgressWriter_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var buf strings.Builder
	pw := &progressWriter{
		writer:   &buf,
		interval: time.Hour,
		ctx:      ctx,
	}

	// Write before cancel should work
	data := []byte("hello")
	n, err := pw.Write(data)
	if err != nil {
		t.Fatalf("write before cancel failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("wrote %d bytes, want %d", n, len(data))
	}

	// Cancel context
	cancel()

	// Write after cancel should fail
	_, err = pw.Write(data)
	if err == nil {
		t.Error("expected error after context cancel")
	}
}

// Verify that the download creates a .part file during download
func TestDownload_PartFile(t *testing.T) {
	testData := make([]byte, 10*1024)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		// Slow write to ensure .part file exists during download
		for i := 0; i < len(testData); i += 1024 {
			w.Write(testData[i : i+1024])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "test.bin")
	tempPath := destPath + ".part"

	mgr := NewManager()

	// Use a channel to check for .part file during download
	partFileSeen := make(chan bool, 1)

	var once syncOnce
	opts := Options{
		URL:      server.URL,
		DestPath: destPath,
		OnProgress: func(p Progress) {
			once.Do(func() {
				// Check if .part file exists
				if _, err := os.Stat(tempPath); err == nil {
					partFileSeen <- true
				} else {
					partFileSeen <- false
				}
			})
		},
		ProgressInterval: 1 * time.Millisecond,
	}

	_, err := mgr.Download(context.Background(), opts)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Check that .part file was seen during download
	select {
	case seen := <-partFileSeen:
		if !seen {
			t.Error(".part file was not seen during download")
		}
	default:
		// If the download was too fast to catch the .part file, that's okay
		// This is a soft check
	}

	// After download, .part file should be gone and final file should exist
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error(".part file should not exist after successful download")
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("final file should exist: %v", err)
	}
}

// syncOnce is a simple once primitive (replaces sync.Once for simplicity)
type syncOnce struct {
	done uint32
}

func (o *syncOnce) Do(f func()) {
	if atomic.CompareAndSwapUint32(&o.done, 0, 1) {
		f()
	}
}

// Ensure progressWriter's Write properly tracks bytes
func TestProgressWriter_Tracking(t *testing.T) {
	var totalBytes int64
	var buf []byte

	pw := &progressWriter{
		writer:   &sliceWriter{&buf},
		interval: time.Hour,
		ctx:      context.Background(),
		onProgress: func(n int64) {
			totalBytes += n
		},
	}

	data := []byte("hello world")
	n, err := pw.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("wrote %d bytes, want %d", n, len(data))
	}
	if totalBytes != int64(len(data)) {
		t.Errorf("total bytes tracked = %d, want %d", totalBytes, len(data))
	}

	// Write more
	data2 := []byte("more data")
	n, err = pw.Write(data2)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if totalBytes != int64(len(data)+len(data2)) {
		t.Errorf("total bytes after second write = %d, want %d", totalBytes, len(data)+len(data2))
	}
}

// sliceWriter is a simple writer that appends to a byte slice
type sliceWriter struct {
	buf *[]byte
}

func (w *sliceWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

// Verify GetProgress works
func TestManager_GetProgress(t *testing.T) {
	testData := make([]byte, 100*1024)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		for i := 0; i < len(testData); i += 1024 {
			w.Write(testData[i : i+1024])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "test.bin")

	mgr := NewManager()

	errCh := make(chan error, 1)
	go func() {
		_, err := mgr.Download(context.Background(), Options{
			URL:      server.URL,
			DestPath: destPath,
		})
		errCh <- err
	}()

	// Wait a bit then check progress
	time.Sleep(20 * time.Millisecond)
	_, ok := mgr.GetProgress(server.URL)
	if !ok {
		t.Error("GetProgress returned false during active download")
	}

	// Wait for completion
	<-errCh

	// After completion, progress should not be available
	_, ok = mgr.GetProgress(server.URL)
	if ok {
		t.Error("GetProgress returned true after download completed")
	}
}
