package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// download fetches url into dir so every sample reads the same local bytes
// and the network stays outside the timer.
func download(dir, url string) (string, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	name := filepath.Base(resp.Request.URL.Path)
	if name == "" || name == "." || name == "/" {
		name = "spec.yaml"
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("fetch %s: %w", url, err)
	}
	return path, f.Close()
}
