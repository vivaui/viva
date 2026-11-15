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
	return client.download(ctx, asset.URL, asset.Size, asset.SHA256, assetFileName(asset), dir, progress)
}

// DownloadPatch streams a delta update into dir and returns the patch file's
// path. It checks the bytes exactly as Download does, against the size and
// SHA-256 the signed feed declared for the patch. The patch is not applied
// here: ReadPatch and PatchReader.Apply do that, after the caller has checked the
// patch names the running application and version.
func (client *Client) DownloadPatch(ctx context.Context, asset Asset, patch Patch, dir string, progress func(done, total int64)) (string, error) {
	if err := asset.validate(); err != nil {
		return "", fmt.Errorf("%w: %w", ErrManifest, err)
	}
	if err := patch.validate(); err != nil {
		return "", fmt.Errorf("%w: %w", ErrManifest, err)
	}
	if patch.Size > client.maxAsset {
		return "", fmt.Errorf("%w: patch is %d bytes, limit is %d", ErrTooLarge, patch.Size, client.maxAsset)
	}
	name := assetFileName(asset) + "." + patch.FromVersion + ".patch"
	return client.download(ctx, patch.URL, patch.Size, patch.SHA256, name, dir, progress)
}

// download fetches one file and keeps it only when its length and digest are
// the declared ones.
func (client *Client) download(ctx context.Context, rawURL string, size int64, digest string, name, dir string, progress func(done, total int64)) (string, error) {
	if size >= math.MaxInt64 {
		return "", fmt.Errorf("%w: file is %d bytes", ErrTooLarge, size)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
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
		return "", fmt.Errorf("%w: %s redirected to %s", ErrInsecureURL, rawURL, response.Request.URL)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update: %s: %s", rawURL, response.Status)
	}

	file, err := os.CreateTemp(dir, ".viva-update-*")
	if err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	temporary := file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(temporary)
		}
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)
	if progress != nil {
		progress(0, size)
	}
	counter := &progressWriter{writer: writer, total: size, progress: progress}
	written, err := io.Copy(counter, io.LimitReader(response.Body, size+1))
	if err != nil {
		return "", fmt.Errorf("update: reading %s: %w", rawURL, err)
	}
	if written != size {
		return "", fmt.Errorf("%w: got %d bytes, feed declares %d", ErrSize, written, size)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(sum, digest) {
		return "", fmt.Errorf("%w: %s", ErrChecksum, sum)
	}
	if err := os.Chmod(temporary, 0o644); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	destination := filepath.Join(dir, name)
	if err := os.Rename(temporary, destination); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	keep = true
	if progress != nil {
		progress(size, size)
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
