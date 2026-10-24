// Package fulltext extracts public article bodies using an isolated browser worker.
package fulltext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func SupportedURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && (u.Hostname() == "openai.com" || u.Hostname() == "www.openai.com") && strings.HasPrefix(u.Path, "/index/")
}

func Fetch(ctx context.Context, nodePath, scriptPath, sourceURL string) (string, error) {
	if !SupportedURL(sourceURL) {
		return "", fmt.Errorf("unsupported article URL")
	}
	if nodePath == "" {
		nodePath = "node"
	}
	if scriptPath == "" {
		scriptPath = "tools/fulltext/fetch.mjs"
	}
	cmd := exec.CommandContext(ctx, nodePath, scriptPath, sourceURL)
	// Give the worker a chance to close Chrome before the hard kill deadline.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("browser worker: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var result struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return "", fmt.Errorf("parse browser output: %w", err)
	}
	if len(result.Content) < 100 || len(result.Content) > 8<<20 {
		return "", fmt.Errorf("invalid article size")
	}
	return result.Content, nil
}
