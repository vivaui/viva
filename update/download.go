package update

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// stallTimeout ends a download that has received nothing for this long. A
// slow connection still delivers something every few seconds; one that is
// silent for a minute has gone away without saying so.
const stallTimeout = time.Minute

// errStalled is the cause a download's context is cancelled with when nothing
// arrived for the stall timeout.
var errStalled = errors.New("update: download stalled")

// Download streams asset into dir and returns the file's path. The bytes are
// hashed as they arrive and the file is kept under its own name only when its
// length and SHA-256 are exactly the ones the feed declared.
//
// A transfer that stops part way, because the connection failed, went silent
// for a minute, the host answered with a temporary error or ctx ended, returns
// an error wrapping ErrInterrupted and keeps the bytes that arrived in a hidden
// file in dir. Calling Download again with the same asset and dir asks the host
// only for the rest; the whole file is still checked against the feed's digest
// before it is kept, and a file that fails that check is fetched again from the
// start once. When dir already holds the verified file, Download returns it
// without a request.
//
// dir must belong to the application: another user who can write to it could
// put a link where the partial file goes. Two processes must not download the
// same asset into the same dir at once; calls within one process wait for each
// other.
//
// progress, when not nil, is called while the file arrives with the bytes
// written so far and the total the feed declared; a resumed download starts at
// the bytes it kept. It runs on the goroutine calling Download, so it may update
// state directly but must not block.
//
// An archive is not unpacked or installed here: this method only guarantees
// the file is the one the signed feed named. A refusal wraps ErrChecksum,
// ErrSize, ErrTooLarge, ErrInsecureURL or ErrManifest, and leaves no partial
// file behind.
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
// path. It checks, keeps and resumes the bytes exactly as Download does, against
// the size and SHA-256 the signed feed declared for the patch. The patch is not
// applied here: ReadPatch and PatchReader.Apply do that, after the caller has
// checked the patch names the running application and version.
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

// download fetches one file, resuming what an earlier call kept, and keeps it
// under name only when its length and digest are the declared ones.
func (client *Client) download(ctx context.Context, rawURL string, size int64, digest string, name, dir string, progress func(done, total int64)) (string, error) {
	if size >= math.MaxInt64 {
		return "", fmt.Errorf("%w: file is %d bytes", ErrTooLarge, size)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	partial := filepath.Join(dir, partialName(digest))
	unlock, err := transfers.lock(ctx, partial)
	if err != nil {
		return "", err
	}
	defer unlock()

	destination := filepath.Join(dir, name)
	if holds(destination, size, digest) {
		report(progress, size, size)
		return destination, nil
	}
	resumed, err := client.fetchFile(ctx, rawURL, size, digest, partial, progress)
	if err != nil && resumed && (errors.Is(err, ErrChecksum) || errors.Is(err, ErrSize)) {
		// The kept bytes were not the start of this file: a crash wrote less than
		// it reported, or the host was serving something else then. The partial is
		// gone, so this asks for the whole file.
		_, err = client.fetchFile(ctx, rawURL, size, digest, partial, progress)
	}
	if err != nil {
		return "", err
	}
	if err := os.Chmod(partial, 0o644); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	if err := os.Rename(partial, destination); err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	report(progress, size, size)
	return destination, nil
}

// fetchFile brings partial up to size bytes, asking the host only for what it does
// not hold yet, and checks the whole file against digest. resumed reports that
// the bytes it checked include some kept from before this call. A refusal
// removes partial; an interruption keeps it for the next call.
func (client *Client) fetchFile(ctx context.Context, rawURL string, size int64, digest, partial string, progress func(done, total int64)) (resumed bool, err error) {
	file, offset, hasher, err := openPartial(partial, size)
	if err != nil {
		return false, err
	}
	refused := false
	defer func() {
		_ = file.Close()
		if refused {
			_ = os.Remove(partial)
		} else if info, statErr := os.Lstat(partial); statErr == nil && info.Size() == 0 {
			_ = os.Remove(partial)
		}
	}()
	resumed = offset > 0
	report(progress, offset, size)

	if offset < size {
		written, err := client.transfer(ctx, rawURL, size, offset, file, hasher, progress)
		offset, resumed = written.size, written.resumed
		if err != nil {
			return resumed, err
		}
	}
	if offset != size {
		refused = true
		return resumed, fmt.Errorf("%w: got %d bytes, feed declares %d", ErrSize, offset, size)
	}
	if err := file.Close(); err != nil {
		return resumed, fmt.Errorf("update: %w", err)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(sum, digest) {
		refused = true
		return resumed, fmt.Errorf("%w: %s", ErrChecksum, sum)
	}
	return resumed, nil
}

// transferred is what a transfer left in the partial file: its length, and
// whether that still starts with bytes kept from an earlier call.
type transferred struct {
	size    int64
	resumed bool
}

// transfer asks the host for the bytes from offset on and appends them to file
// and hasher. A host that ignores the range, or answers for another part of the
// file, makes it start over from the first byte. Nothing arriving for the
// client's stall timeout ends it with ErrInterrupted, as does a failed
// connection, a temporary status or the end of ctx.
func (client *Client) transfer(ctx context.Context, rawURL string, size, offset int64, file *os.File, hasher hash.Hash, progress func(done, total int64)) (transferred, error) {
	state := transferred{size: offset, resumed: offset > 0}
	watched, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	stall := client.stall
	if stall <= 0 {
		stall = stallTimeout
	}
	watchdog := time.AfterFunc(stall, func() { stop(errStalled) })
	defer watchdog.Stop()
	alive := func() { watchdog.Reset(stall) }
	interrupted := func(err error) error {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %w", ErrInterrupted, ctx.Err())
		}
		if errors.Is(context.Cause(watched), errStalled) {
			return fmt.Errorf("%w: %s: nothing arrived for %s", ErrInterrupted, rawURL, stall)
		}
		return fmt.Errorf("%w: %s: %w", ErrInterrupted, rawURL, err)
	}
	startOver := func() error {
		state = transferred{}
		hasher.Reset()
		if err := file.Truncate(0); err != nil {
			return fmt.Errorf("update: %w", err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("update: %w", err)
		}
		report(progress, 0, size)
		return nil
	}

	var response *http.Response
	for {
		var err error
		response, err = client.requestFrom(watched, rawURL, state.size)
		if err != nil {
			if errors.Is(err, ErrInsecureURL) || !transient(err) {
				return state, err
			}
			return state, interrupted(err)
		}
		alive()
		status := response.StatusCode
		if status == http.StatusPartialContent && state.size > 0 && rangeFrom(response.Header.Get("Content-Range"), state.size, size) {
			break
		}
		if status == http.StatusOK {
			if state.size > 0 {
				// The host ignored the range and sent the whole file.
				if err := startOver(); err != nil {
					_ = response.Body.Close()
					return state, err
				}
			}
			break
		}
		_ = response.Body.Close()
		if state.size > 0 && (status == http.StatusPartialContent || status == http.StatusRequestedRangeNotSatisfiable) {
			// An answer for another part of the file, or none: the kept bytes
			// cannot be continued, so ask once more for all of it.
			if err := startOver(); err != nil {
				return state, err
			}
			continue
		}
		failure := fmt.Errorf("update: %s: %s", rawURL, response.Status)
		if temporaryStatus(status) {
			return state, interrupted(failure)
		}
		return state, failure
	}
	defer response.Body.Close()

	body := &watchedReader{reader: response.Body, alive: alive}
	counter := &progressWriter{writer: io.MultiWriter(file, hasher), total: size, done: state.size, progress: progress}
	written, err := io.Copy(counter, io.LimitReader(body, size-state.size+1))
	state.size += written
	if err != nil {
		if body.failed {
			return state, interrupted(err)
		}
		return state, fmt.Errorf("update: writing %s: %w", file.Name(), err)
	}
	return state, nil
}

// requestFrom asks for rawURL, from byte offset on when offset is not zero.
func (client *Client) requestFrom(ctx context.Context, rawURL string, offset int64) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	request.Header.Set("User-Agent", "viva-update/1")
	if offset > 0 {
		request.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	response, err := client.assetHTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	if response.Request != nil && response.Request.URL != nil && response.Request.URL.Scheme != "https" {
		_ = response.Body.Close()
		return nil, fmt.Errorf("%w: %s redirected to %s", ErrInsecureURL, rawURL, response.Request.URL)
	}
	return response, nil
}

// openPartial opens the file a download is kept in while it arrives and hashes
// what it already holds. Anything there that is not a regular file is removed,
// and a file longer than size is emptied: neither can be the start of this one.
func openPartial(partial string, size int64) (*os.File, int64, hash.Hash, error) {
	if info, err := os.Lstat(partial); err == nil && !info.Mode().IsRegular() {
		if err := os.Remove(partial); err != nil {
			return nil, 0, nil, fmt.Errorf("update: %w", err)
		}
	}
	file, err := os.OpenFile(partial, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("update: %w", err)
	}
	hasher := sha256.New()
	offset, err := io.Copy(hasher, io.LimitReader(file, size+1))
	if err == nil && offset > size {
		offset = 0
		hasher.Reset()
		err = file.Truncate(0)
	}
	if err == nil {
		_, err = file.Seek(offset, io.SeekStart)
	}
	if err != nil {
		_ = file.Close()
		return nil, 0, nil, fmt.Errorf("update: %w", err)
	}
	return file, offset, hasher, nil
}

// partialName is the hidden file a download is kept in until it is complete.
// It is named by the digest, so bytes are only ever continued with the rest of
// the same file.
func partialName(digest string) string {
	return ".viva-update-" + strings.ToLower(digest) + ".part"
}

// holds reports whether path is a regular file of exactly size bytes whose
// SHA-256 is digest.
func holds(path string, size int64, digest string) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hasher := sha256.New()
	if written, err := io.Copy(hasher, io.LimitReader(file, size+1)); err != nil || written != size {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), digest)
}

// rangeFrom reports whether a Content-Range header answers a request for the
// bytes from offset to the end of a file of size bytes, and nothing else.
func rangeFrom(header string, offset, size int64) bool {
	spec, ok := strings.CutPrefix(header, "bytes ")
	if !ok {
		return false
	}
	span, complete, ok := strings.Cut(spec, "/")
	if !ok || (complete != "*" && complete != strconv.FormatInt(size, 10)) {
		return false
	}
	first, last, ok := strings.Cut(span, "-")
	return ok && first == strconv.FormatInt(offset, 10) && last == strconv.FormatInt(size-1, 10)
}

// temporaryStatus reports a status a host answers with while it is busy or
// restarting, after which the same request may succeed.
func temporaryStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// transient reports whether a failed request may succeed when tried again. A
// certificate the system does not trust will not start being trusted.
func transient(err error) bool {
	var certificate *tls.CertificateVerificationError
	return !errors.As(err, &certificate)
}

func report(progress func(done, total int64), done, total int64) {
	if progress != nil {
		progress(done, total)
	}
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
	report(writer.progress, writer.done, writer.total)
	return written, err
}

// watchedReader tells alive whenever bytes arrive and remembers that the
// connection, rather than the file being written, failed.
type watchedReader struct {
	reader io.Reader
	alive  func()
	failed bool
}

func (reader *watchedReader) Read(data []byte) (int, error) {
	n, err := reader.reader.Read(data)
	if n > 0 {
		reader.alive()
	}
	if err != nil && err != io.EOF {
		reader.failed = true
	}
	return n, err
}

// transfers lets one download at a time use a partial file, so two calls in
// this process for the same file in the same directory take turns; the second
// then finds the first one's verified file.
var transfers keyedLock

type keyedLock struct {
	mu   sync.Mutex
	held map[string]chan struct{}
}

func (locks *keyedLock) lock(ctx context.Context, key string) (func(), error) {
	if absolute, err := filepath.Abs(key); err == nil {
		key = absolute
	}
	for {
		locks.mu.Lock()
		wait, busy := locks.held[key]
		if !busy {
			if locks.held == nil {
				locks.held = make(map[string]chan struct{})
			}
			done := make(chan struct{})
			locks.held[key] = done
			locks.mu.Unlock()
			return func() {
				locks.mu.Lock()
				delete(locks.held, key)
				locks.mu.Unlock()
				close(done)
			}, nil
		}
		locks.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ErrInterrupted, ctx.Err())
		}
	}
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
