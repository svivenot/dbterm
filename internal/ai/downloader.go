package ai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ModelInfo describes a downloadable LLM model
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	SizeDisplay string `json:"size_display"`
	SizeBytes   int64  `json:"size_bytes"`
	Description string `json:"description"`
}

var (
	Qwen2_5_Coder_1_5B = ModelInfo{
		ID:          "qwen2.5-coder-1.5b-instruct",
		Name:        "Qwen 2.5 Coder 1.5B Instruct (Q4_K_M)",
		Filename:    "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
		URL:         "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
		SizeDisplay: "986 MB",
		SizeBytes:   1034250240,
		Description: "Compact SQL model. Ultra-fast, low memory footprint.",
	}

	Qwen2_5_Coder_3B = ModelInfo{
		ID:          "qwen2.5-coder-3b-instruct",
		Name:        "Qwen 2.5 Coder 3B Instruct (Q4_K_M)",
		Filename:    "qwen2.5-coder-3b-instruct-q4_k_m.gguf",
		URL:         "https://huggingface.co/Qwen/Qwen2.5-Coder-3B-Instruct-GGUF/resolve/main/qwen2.5-coder-3b-instruct-q4_k_m.gguf",
		SizeDisplay: "2.1 GB",
		SizeBytes:   2224000000,
		Description: "Recommended: Exceptional SQL & business logic reasoning for enterprise databases.",
	}

	Qwen2_5_Coder_7B = ModelInfo{
		ID:          "qwen2.5-coder-7b-instruct",
		Name:        "Qwen 2.5 Coder 7B Instruct (Q4_K_M)",
		Filename:    "qwen2.5-coder-7b-instruct-q4_k_m.gguf",
		URL:         "https://huggingface.co/Qwen/Qwen2.5-Coder-7B-Instruct-GGUF/resolve/main/qwen2.5-coder-7b-instruct-q4_k_m.gguf",
		SizeDisplay: "4.7 GB",
		SizeBytes:   5024000000,
		Description: "Top-tier SQL intelligence, matches GPT-4o-mini on complex multi-table joins & CTEs.",
	}

	SQLCoder_7B_2 = ModelInfo{
		ID:          "sqlcoder-7b-2",
		Name:        "Defog SQLCoder 7B-2 (Q4_K_M)",
		Filename:    "sqlcoder-7b-2.Q4_K_M.gguf",
		URL:         "https://huggingface.co/TheBloke/sqlcoder-7b-2-GGUF/resolve/main/sqlcoder-7b-2.Q4_K_M.gguf",
		SizeDisplay: "4.5 GB",
		SizeBytes:   4480000000,
		Description: "Defog specialized Text-to-SQL foundation model fine-tuned specifically on database query generation.",
	}

	DefaultModel = SQLCoder_7B_2

	AvailableModels = []ModelInfo{
		SQLCoder_7B_2,
		Qwen2_5_Coder_3B,
		Qwen2_5_Coder_7B,
		Qwen2_5_Coder_1_5B,
	}
)

// DownloadProgress reports real-time download status
type DownloadProgress struct {
	BytesRead     int64
	TotalBytes    int64
	Percentage    float64
	SpeedBytesSec int64
	EtaSeconds    int
	Done          bool
	Error         error
}

var (
	trackerMu      sync.RWMutex
	activeProgress DownloadProgress
	isDownloading  bool
)

// GetGlobalDownloadProgress returns a thread-safe snapshot of the current download progress
func GetGlobalDownloadProgress() DownloadProgress {
	trackerMu.RLock()
	defer trackerMu.RUnlock()
	return activeProgress
}

// IsDownloading returns true if a background download is currently active
func IsDownloading() bool {
	trackerMu.RLock()
	defer trackerMu.RUnlock()
	return isDownloading
}

// StartBackgroundDownload starts the download in a non-blocking background goroutine
func StartBackgroundDownload(ctx context.Context, modelInfo ModelInfo) {
	trackerMu.Lock()
	if isDownloading {
		trackerMu.Unlock()
		return
	}
	isDownloading = true
	activeProgress = DownloadProgress{
		TotalBytes: modelInfo.SizeBytes,
		Percentage: 0,
	}
	trackerMu.Unlock()

	go func() {
		defer func() {
			trackerMu.Lock()
			isDownloading = false
			trackerMu.Unlock()
		}()

		path, err := DownloadModel(ctx, modelInfo, func(p DownloadProgress) {
			trackerMu.Lock()
			activeProgress = p
			trackerMu.Unlock()
		})

		trackerMu.Lock()
		if err != nil {
			activeProgress.Error = err
			activeProgress.Done = true
		} else {
			activeProgress.Done = true
			activeProgress.Percentage = 100
			activeProgress.BytesRead = activeProgress.TotalBytes
		}
		trackerMu.Unlock()
		_ = path
	}()
}

// GetModelsDir returns the standard storage directory for GGUF models
func GetModelsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "dbterm", "models")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// GetModelFilePath returns the full absolute path of the model file
func GetModelFilePath(modelInfo ModelInfo) (string, error) {
	dir, err := GetModelsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, modelInfo.Filename), nil
}

// IsModelInstalled checks if the model GGUF file exists on disk
func IsModelInstalled(modelInfo ModelInfo) bool {
	path, err := GetModelFilePath(modelInfo)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Verify non-trivial size (> 50 MB)
	return info.Size() > 50*1024*1024
}

// DownloadModel downloads the model file from Hugging Face with real-time progress callbacks
func DownloadModel(ctx context.Context, modelInfo ModelInfo, progressFn func(DownloadProgress)) (string, error) {
	destPath, err := GetModelFilePath(modelInfo)
	if err != nil {
		return "", fmt.Errorf("failed to get model directory: %w", err)
	}

	tmpPath := destPath + ".downloading"

	// Check existing partial download
	var startBytes int64 = 0
	if stat, err := os.Stat(tmpPath); err == nil {
		startBytes = stat.Size()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", modelInfo.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "dbterm/1.0 (Embedded-SQL-AI)")

	if startBytes > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startBytes))
	}

	client := &http.Client{
		Timeout: 0, // No global timeout for large downloads; context controls cancellation
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to initiate download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("server returned status %s (%d)", resp.Status, resp.StatusCode)
	}

	totalBytes := resp.ContentLength
	if startBytes > 0 && resp.StatusCode == http.StatusPartialContent {
		totalBytes += startBytes
	} else if totalBytes <= 0 {
		totalBytes = modelInfo.SizeBytes
	}

	flags := os.O_CREATE | os.O_WRONLY
	if startBytes > 0 && resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		startBytes = 0
	}

	file, err := os.OpenFile(tmpPath, flags, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open output file: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 256*1024) // 256KB buffer for high throughput
	bytesRead := startBytes
	lastReport := time.Now()
	lastBytes := bytesRead

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return "", fmt.Errorf("failed to write to file: %w", writeErr)
			}
			bytesRead += int64(n)
		}

		now := time.Now()
		if progressFn != nil && (now.Sub(lastReport) >= 100*time.Millisecond || readErr != nil) {
			elapsedSec := now.Sub(lastReport).Seconds()
			var speed int64 = 0
			if elapsedSec > 0 {
				speed = int64(float64(bytesRead-lastBytes) / elapsedSec)
			}

			pct := float64(0)
			if totalBytes > 0 {
				pct = float64(bytesRead) / float64(totalBytes) * 100.0
			}

			etaSec := 0
			if speed > 0 && totalBytes > bytesRead {
				etaSec = int((totalBytes - bytesRead) / speed)
			}

			progressFn(DownloadProgress{
				BytesRead:     bytesRead,
				TotalBytes:    totalBytes,
				Percentage:    pct,
				SpeedBytesSec: speed,
				EtaSeconds:    etaSec,
				Done:          readErr == io.EOF,
				Error:         nil,
			})

			lastReport = now
			lastBytes = bytesRead
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return "", fmt.Errorf("download error: %w", readErr)
		}
	}

	_ = file.Close()

	// Atomic rename to final path
	if err := os.Rename(tmpPath, destPath); err != nil {
		return "", fmt.Errorf("failed to finalize model file: %w", err)
	}

	if progressFn != nil {
		progressFn(DownloadProgress{
			BytesRead:     bytesRead,
			TotalBytes:    totalBytes,
			Percentage:    100.0,
			SpeedBytesSec: 0,
			EtaSeconds:    0,
			Done:          true,
			Error:         nil,
		})
	}

	return destPath, nil
}
