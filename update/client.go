package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultMaxManifestBytes bounds latest.json and its signature. A feed is a
	// few kilobytes; anything larger is not one.
	DefaultMaxManifestBytes int64 = 1 << 20
	// DefaultMaxAssetBytes bounds an archive Download writes when Config does
	// not say otherwise.
	DefaultMaxAssetBytes int64 = 512 << 20
	// maxSignatureBytes bounds latest.json.sig. An Ed25519 signature is 64
	// bytes, which is 88 in base64.
	maxSignatureBytes int64 = 4 << 10
)

var (
	// ErrManifest reports a feed this package refuses: one it cannot read, or one
	// that names a schema, app, version, platform or archive it must not act on.
	ErrManifest = errors.New("update: invalid feed")
	// ErrSignature reports a feed whose latest.json.sig does not verify with
	// any configured key, or both files disagree.
	ErrSignature = errors.New("update: feed signature does not verify")
	// ErrNoAsset reports a feed with no archive for the running platform.
	ErrNoAsset = errors.New("update: no download for this platform")
	// ErrInsecureURL reports a URL that is not HTTPS, or a redirect that left
	// HTTPS.
	ErrInsecureURL = errors.New("update: URL must be https")
	// ErrTooLarge reports a response larger than the configured limit or the
	// size the feed declared.
	ErrTooLarge = errors.New("update: response is larger than allowed")
	// ErrSize reports a download whose length is not the one the feed declared.
	ErrSize = errors.New("update: download does not match its declared size")
	// ErrChecksum reports a download whose SHA-256 is not the one the feed
	// declared.
	ErrChecksum = errors.New("update: download does not match its checksum")
)

// Config configures a Client. FeedURL, PublicKeys and Current are required.
type Config struct {
	// FeedURL is the HTTPS address of latest.json. The signature is read from
	// FeedURL + ".sig".
	FeedURL string
	// PublicKeys are the Ed25519 keys a feed may be signed with. More than one
	// lets a key be rotated without an old release refusing the next feed.
	PublicKeys []ed25519.PublicKey
	// App, when set, must equal the feed's app identifier; a feed signed for
	// another application is refused.
	App string
	// Channel, when set, must equal the feed's channel.
	Channel string
	// Current is the running version, such as "1.4.1".
	Current string
	// HTTP overrides the client used for the feed and its signature. Nil uses a
	// client with a 30 second timeout. The archive download always uses a client
	// with no total timeout, because a large installer may take longer than any
	// sensible request timeout; the context passed to Download cancels it.
	HTTP *http.Client
	// MaxAssetBytes bounds Download. Zero means DefaultMaxAssetBytes.
	MaxAssetBytes int64
}

// Client reads one application's update feed. It performs no I/O until Check
// or Download is called and is safe for concurrent use.
type Client struct {
	feedURL      string
	signatureURL string
	keys         []ed25519.PublicKey
	app          string
	channel      string
	current      string
	feedHTTP     *http.Client
	assetHTTP    *http.Client
	maxAsset     int64
	maxManifest  int64
}

// New validates config and returns a Client.
func New(config Config) (*Client, error) {
	if err := checkHTTPS(config.FeedURL); err != nil {
		return nil, err
	}
	if len(config.PublicKeys) == 0 {
		return nil, fmt.Errorf("update: no public keys")
	}
	keys := make([]ed25519.PublicKey, 0, len(config.PublicKeys))
	for index, key := range config.PublicKeys {
		if len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("update: public key %d is %d bytes, want %d", index, len(key), ed25519.PublicKeySize)
		}
		keys = append(keys, bytes.Clone(key))
	}
	if _, err := parseVersion(config.Current); err != nil {
		return nil, err
	}
	feedHTTP := config.HTTP
	assetHTTP := config.HTTP
	if feedHTTP == nil {
		feedHTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if assetHTTP == nil {
		// No total timeout: Download's context is what bounds it, and an archive
		// may legitimately take longer than a feed request.
		assetHTTP = &http.Client{}
	}
	maxAsset := config.MaxAssetBytes
	if maxAsset <= 0 {
		maxAsset = DefaultMaxAssetBytes
	}
	signatureURL, err := signatureURLFor(config.FeedURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		feedURL:      config.FeedURL,
		signatureURL: signatureURL,

		keys:        keys,
		app:         config.App,
		channel:     config.Channel,
		current:     config.Current,
		feedHTTP:    feedHTTP,
		assetHTTP:   assetHTTP,
		maxAsset:    maxAsset,
		maxManifest: DefaultMaxManifestBytes,
	}, nil
}

// Check fetches the feed, verifies its signature and returns the newest
// release when it is newer than Config.Current. When the feed carries nothing
// newer it returns (nil, nil), which is not an error: the application is up to
// date.
//
// A refusal wraps ErrSignature, ErrManifest, ErrInsecureURL or ErrTooLarge.
func (client *Client) Check(ctx context.Context) (*Feed, error) {
	body, err := client.fetch(ctx, client.feedURL, client.maxManifest)
	if err != nil {
		return nil, err
	}
	signatureBytes, err := client.fetch(ctx, client.signatureURL, maxSignatureBytes)
	if err != nil {
		return nil, err
	}
	signature, err := decodeSignature(signatureBytes)
	if err != nil {
		return nil, err
	}
	if !verifyAny(client.keys, body, signature) {
		return nil, ErrSignature
	}
	var feed Feed
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrManifest, err)
	}
	if err := feed.validate(); err != nil {
		return nil, err
	}
	if client.app != "" && feed.App != client.app {
		return nil, fmt.Errorf("%w: feed is for %q, not %q", ErrManifest, feed.App, client.app)
	}
	if client.channel != "" && feed.Channel != client.channel {
		return nil, fmt.Errorf("%w: feed channel is %q, not %q", ErrManifest, feed.Channel, client.channel)
	}
	order, err := Compare(feed.Latest.Version, client.current)
	if err != nil {
		return nil, fmt.Errorf("%w: latest version: %v", ErrManifest, err)
	}
	if order <= 0 {
		return nil, nil
	}
	return &feed, nil
}

// fetch reads one HTTPS document, refusing anything over limit bytes and any
// redirect that leaves HTTPS.
func (client *Client) fetch(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	if err := checkHTTPS(rawURL); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	request.Header.Set("User-Agent", "viva-update/1")
	response, err := client.feedHTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	defer response.Body.Close()
	if response.Request != nil && response.Request.URL != nil && response.Request.URL.Scheme != "https" {
		return nil, fmt.Errorf("%w: %s redirected to %s", ErrInsecureURL, rawURL, response.Request.URL)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: %s: %s", rawURL, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("update: reading %s: %w", rawURL, err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: %s", ErrTooLarge, rawURL)
	}
	return body, nil
}

// signatureURLFor puts ".sig" after the feed's path, so a feed URL with a
// query keeps it (some hosts need one to address a release).
func signatureURLFor(feedURL string) (string, error) {
	parsed, err := url.Parse(feedURL)
	if err != nil {
		return "", fmt.Errorf("update: %w", err)
	}
	parsed.Path += ".sig"
	return parsed.String(), nil
}

func decodeSignature(text []byte) ([]byte, error) {
	encoded := strings.TrimSpace(string(text))
	if encoded == "" {
		return nil, fmt.Errorf("%w: signature is empty", ErrSignature)
	}
	signature, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		signature, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: signature is not base64", ErrSignature)
	}
	if len(signature) != ed25519.SignatureSize {
		return nil, fmt.Errorf("%w: signature is %d bytes, want %d", ErrSignature, len(signature), ed25519.SignatureSize)
	}
	return signature, nil
}

func verifyAny(keys []ed25519.PublicKey, body, signature []byte) bool {
	for _, key := range keys {
		if ed25519.Verify(key, body, signature) {
			return true
		}
	}
	return false
}
