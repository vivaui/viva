package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Download streams asset into dir and returns the file's path. The bytes are
// hashed as they arrive and the call fails without keeping a file unless the
// length and SHA-256 are exactly the ones the feed declared; a partially
// written download is removed.
//
// progress, when not nil, is called while the file arrives with the bytes
// written so far and the total the feed declared. It runs on the goroutine
// calling Download, so it may update state directly but must not block.
//
// An archive is not unpacked or installed here: this method only guarantees
// the file is the one the signed feed named. A refusal wraps ErrChecksum,
// ErrSize, ErrTooLarge, ErrInsecureURL or ErrManifest.
func (client *Client) Download(ctx context.Context, asset Asset, dir string, progress func(done, total int64)) (string, error) {
	if err := asset.validate(); err != nil {
		return "", fmt.Errorf("%w: %w", ErrManifest, err)
	}
	if asset.Size > client.maxAsset {
		return "", fmt.Errorf("%w: archive is %d bytes, limit is %d", ErrTooLarge, asset.Size, client.maxAsset)
	}
	if asset.Size >= math.MaxInt64 {
		return "", fmt.Errorf("%w: archive is %d bytes", ErrTooLarge, asset.Size)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	request.Header.Set("User-Agent", "viva-update/1")
	response, err := client.assetHTTP.Do(request)
	if err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	defer response.Body.Close()
	if response.Request != nil && response.Request.URL != nil && response.Request.URL.Scheme != "https" {
		return "", fmt.Errorf("%w: %s redirected to %s", ErrInsecureURL, asset.URL, response.Request.URL)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update: %s: %s", asset.URL, response.Status)
	}

	file, err := os.CreateTemp(dir, ".viva-update-*")
	if err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	name := file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(name)
		}
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)
	if progress != nil {
		progress(0, asset.Size)
	}
	counter := &progressWriter{writer: writer, total: asset.Size, progress: progress}
	written, err := io.Copy(counter, io.LimitReader(response.Body, asset.Size+1))
	if err != nil {
		return "", fmt.Errorf("update: reading %s: %w", asset.URL, err)
	}
	if written != asset.Size {
		return "", fmt.Errorf("%w: got %d bytes, feed declares %d", ErrSize, written, asset.Size)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(digest, asset.SHA256) {
		return "", fmt.Errorf("%w: %s", ErrChecksum, digest)
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	destination := filepath.Join(dir, assetFileName(asset))
	if err := os.Rename(name, destination); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	keep = true
	if progress != nil {
		progress(asset.Size, asset.Size)
	}
	return destination, nil
}

type progressWriter struct {
	writer   io.Writer
	total    int64
	done     int64
	progress func(done, total int64)
}

func (writer *progressWriter) Write(data []byte) (int, error) {
	written, err := writer.writer.Write(data)
	writer.done += int64(written)
	if writer.progress != nil {
		writer.progress(writer.done, writer.total)
	}
	return written, err
}

// assetFileName is the name an archive is saved under: the last element of
// its URL path, or a fixed name when that is not a plain file name. A backslash
// is a path separator on Windows even when the URL wrote "%5C", so the name is
// reduced with Base and refused when a separator or a Windows-reserved
// character survives.
func assetFileName(asset Asset) string {
	parsed, err := url.Parse(asset.URL)
	if err == nil {
		name := path.Base(strings.ReplaceAll(parsed.Path, "\\", "/"))
		if validAssetFileName(name) {
			return name
		}
	}
	return "update-archive"
}

func validAssetFileName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, "/\\:*?\"<>|\x00")
}
