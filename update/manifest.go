package update

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// SchemaVersion is the only feed schema this package reads. A feed that names
// another schema is refused rather than guessed at.
const SchemaVersion = 1

// Archive formats a feed may list. They are the archives `viva package`
// makes: a macOS .app in a zip, a Windows NSIS installer, or a Linux
// executable in a tar.gz.
const (
	FormatZIP   = "zip"
	FormatNSIS  = "nsis"
	FormatTarGz = "tar.gz"
)

// Feed is one signed update document. latest.json holds exactly this value.
type Feed struct {
	// Schema is SchemaVersion.
	Schema int `json:"schema"`
	// App identifies the application the feed belongs to, such as
	// "dev.viva.gallery". A client configured with an app name refuses a feed
	// whose App differs.
	App string `json:"app"`
	// Channel names the release channel the feed carries, such as "stable" or
	// "beta". It is optional; empty means the feed's only channel.
	Channel string `json:"channel,omitempty"`
	// Expires, when set, is when the client stops trusting this feed. The
	// signature covers the document, not the time it is served, so a feed with
	// no expiry can be replayed by anyone who can answer the request, offering a
	// release that was since superseded because it was found vulnerable. It is
	// optional: the zero value means the feed never expires.
	Expires time.Time `json:"expires,omitzero"`
	// Latest is the newest release the feed knows about.
	Latest Release `json:"latest"`
}

// Release is one application version and the archives that carry it.
type Release struct {
	Version string    `json:"version"`
	Date    time.Time `json:"date"`
	// NotesURL, when set, is an HTTPS page describing the release.
	NotesURL string `json:"notes_url,omitempty"`
	// MinOS optionally names the oldest operating system each platform needs,
	// such as {"darwin": "12.0"}. This package carries the field and leaves the
	// comparison to the application.
	MinOS map[string]string `json:"min_os,omitempty"`
	// Assets lists the archives of this release. At least one is required.
	Assets []Asset `json:"assets"`
}

// Asset is one downloadable archive.
type Asset struct {
	// OS and Arch are runtime.GOOS and runtime.GOARCH values: darwin, windows
	// or linux, and amd64 or arm64.
	OS   string `json:"os"`
	Arch string `json:"arch"`
	// Format is FormatZIP, FormatNSIS or FormatTarGz.
	Format string `json:"format"`
	// URL is the HTTPS address of the archive.
	URL string `json:"url"`
	// Size is the archive's exact length in bytes.
	Size int64 `json:"size"`
	// SHA256 is the archive's lowercase hexadecimal SHA-256 digest.
	SHA256 string `json:"sha256"`
	// Patches are updates from earlier versions, each a delta against the
	// installed payload. A client that does not read them downloads the
	// archive.
	Patches []Patch `json:"patches,omitempty"`
}

// Patch is one delta update for an asset.
type Patch struct {
	// FromVersion is the installed version the patch applies to.
	FromVersion string `json:"from_version"`
	// URL is the HTTPS address of the patch file.
	URL string `json:"url"`
	// Size is the patch's exact length in bytes.
	Size int64 `json:"size"`
	// SHA256 is the patch's lowercase hexadecimal SHA-256 digest.
	SHA256 string `json:"sha256"`
}

// PatchFrom returns the patch that applies to version, if the feed offers one.
func (asset Asset) PatchFrom(version string) (Patch, bool) {
	for _, patch := range asset.Patches {
		if patch.FromVersion == version {
			return patch, true
		}
	}
	return Patch{}, false
}

// Asset returns the archive this feed offers for goos and goarch. When a
// platform has more than one format, the one an application should install
// first wins: the NSIS installer over a Windows zip, the zip on macOS and the
// tar.gz on Linux. An unsupported platform reports ErrNoAsset.
func (feed Feed) Asset(goos, goarch string) (Asset, error) {
	best := Asset{}
	found := false
	for _, asset := range feed.Latest.Assets {
		if asset.OS != goos || asset.Arch != goarch {
			continue
		}
		if !found || assetRank(asset.OS, asset.Format) < assetRank(best.OS, best.Format) {
			best, found = asset, true
		}
	}
	if !found {
		return Asset{}, fmt.Errorf("%w: %s/%s", ErrNoAsset, goos, goarch)
	}
	return best, nil
}

func assetRank(goos, format string) int {
	switch {
	case goos == "windows" && format == FormatNSIS:
		return 0
	case format == FormatZIP:
		return 1
	case format == FormatTarGz:
		return 2
	}
	return 3
}

// validate refuses a document this package must not act on. Every error wraps
// ErrManifest.
func (feed Feed) validate() error {
	if feed.Schema != SchemaVersion {
		return fmt.Errorf("%w: schema is %d, want %d", ErrManifest, feed.Schema, SchemaVersion)
	}
	if strings.TrimSpace(feed.App) == "" {
		return fmt.Errorf("%w: app is empty", ErrManifest)
	}
	if strings.ContainsAny(feed.Channel, " \t\r\n") {
		return fmt.Errorf("%w: channel %q contains a space", ErrManifest, feed.Channel)
	}
	if _, err := parseVersion(feed.Latest.Version); err != nil {
		return fmt.Errorf("%w: latest: %v", ErrManifest, err)
	}
	if !feed.Expires.IsZero() && !feed.Expires.After(feed.Latest.Date) {
		return fmt.Errorf("%w: expires is not after the release date", ErrManifest)
	}
	if feed.Latest.NotesURL != "" {
		if err := checkHTTPS(feed.Latest.NotesURL); err != nil {
			return fmt.Errorf("%w: notes_url: %v", ErrManifest, err)
		}
	}
	for operatingSystem := range feed.Latest.MinOS {
		switch operatingSystem {
		case "darwin", "windows", "linux":
		default:
			return fmt.Errorf("%w: min_os names %q", ErrManifest, operatingSystem)
		}
	}
	if len(feed.Latest.Assets) == 0 {
		return fmt.Errorf("%w: latest has no assets", ErrManifest)
	}
	seen := make(map[string]bool, len(feed.Latest.Assets))
	for index, asset := range feed.Latest.Assets {
		if err := asset.validate(); err != nil {
			return fmt.Errorf("%w: asset %d: %v", ErrManifest, index, err)
		}
		key := asset.OS + "/" + asset.Arch + "/" + asset.Format
		if seen[key] {
			return fmt.Errorf("%w: asset %s appears twice", ErrManifest, key)
		}
		seen[key] = true
	}
	return nil
}

func (asset Asset) validate() error {
	switch asset.OS {
	case "darwin", "windows", "linux":
	default:
		return fmt.Errorf("os %q is not darwin, windows or linux", asset.OS)
	}
	switch asset.Arch {
	case "amd64", "arm64":
	default:
		return fmt.Errorf("arch %q is not amd64 or arm64", asset.Arch)
	}
	switch asset.Format {
	case FormatZIP, FormatNSIS, FormatTarGz:
	default:
		return fmt.Errorf("format %q is not zip, nsis or tar.gz", asset.Format)
	}
	switch asset.OS {
	case "darwin":
		if asset.Format != FormatZIP {
			return fmt.Errorf("darwin installs a zip, not %q", asset.Format)
		}
	case "windows":
		if asset.Format != FormatNSIS && asset.Format != FormatZIP {
			return fmt.Errorf("windows installs an nsis installer or a zip, not %q", asset.Format)
		}
	case "linux":
		if asset.Format != FormatTarGz {
			return fmt.Errorf("linux installs a tar.gz, not %q", asset.Format)
		}
	}
	if err := checkHTTPS(asset.URL); err != nil {
		return err
	}
	if asset.Size <= 0 {
		return fmt.Errorf("size must be positive")
	}
	if len(asset.SHA256) != 64 {
		return fmt.Errorf("sha256 must be 64 hexadecimal digits")
	}
	if _, err := hex.DecodeString(asset.SHA256); err != nil {
		return fmt.Errorf("sha256: %v", err)
	}
	seenPatches := make(map[string]bool, len(asset.Patches))
	for _, patch := range asset.Patches {
		if err := patch.validate(); err != nil {
			return err
		}
		if seenPatches[patch.FromVersion] {
			return fmt.Errorf("patches from %s appear twice", patch.FromVersion)
		}
		seenPatches[patch.FromVersion] = true
	}
	return nil
}

func (patch Patch) validate() error {
	if _, err := parseVersion(patch.FromVersion); err != nil {
		return fmt.Errorf("patch from_version: %v", err)
	}
	if patch.FromVersion == "" {
		return fmt.Errorf("patch from_version is empty")
	}
	if err := checkHTTPS(patch.URL); err != nil {
		return fmt.Errorf("patch from %s: %w", patch.FromVersion, err)
	}
	if patch.Size <= 0 {
		return fmt.Errorf("patch from %s: size must be positive", patch.FromVersion)
	}
	if len(patch.SHA256) != 64 || strings.ToLower(patch.SHA256) != patch.SHA256 {
		return fmt.Errorf("patch from %s: sha256 must be 64 lowercase hexadecimal digits", patch.FromVersion)
	}
	if _, err := hex.DecodeString(patch.SHA256); err != nil {
		return fmt.Errorf("patch from %s: sha256: %v", patch.FromVersion, err)
	}
	return nil
}

// checkHTTPS is the one place both the feed and its archives are checked: an
// update that arrives over plaintext can be replaced by anyone on the path.
func checkHTTPS(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInsecureURL, err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%w: %q", ErrInsecureURL, raw)
	}
	if parsed.User != nil {
		return fmt.Errorf("%w: %q has credentials", ErrInsecureURL, raw)
	}
	return nil
}
