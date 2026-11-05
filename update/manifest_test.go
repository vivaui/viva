package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func testDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func validFeed() Feed {
	asset := func(goos, goarch, format string) Asset {
		return Asset{
			OS:     goos,
			Arch:   goarch,
			Format: format,
			URL:    "https://example.com/app-" + goos + "-" + goarch + "." + format,
			Size:   1024,
			SHA256: testDigest(goos + goarch + format),
		}
	}
	return Feed{
		Schema:  SchemaVersion,
		App:     "dev.viva.gallery",
		Channel: "stable",
		Expires: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
		Latest: Release{
			Version:  "1.4.2",
			Date:     time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC),
			NotesURL: "https://example.com/notes/1.4.2",
			MinOS:    map[string]string{"darwin": "12.0", "windows": "10.0", "linux": "0"},
			Assets: []Asset{
				asset("darwin", "arm64", FormatZIP),
				asset("windows", "amd64", FormatZIP),
				asset("windows", "amd64", FormatNSIS),
				asset("linux", "arm64", FormatTarGz),
			},
		},
	}
}

func TestFeedValidates(t *testing.T) {
	if err := validFeed().validate(); err != nil {
		t.Fatalf("valid feed: %v", err)
	}
}

func TestFeedRefusesWhatItMustNotActOn(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Feed)
		want   string
	}{
		{"schema", func(feed *Feed) { feed.Schema = 2 }, "schema"},
		{"empty app", func(feed *Feed) { feed.App = "  " }, "app is empty"},
		{"channel space", func(feed *Feed) { feed.Channel = "a b" }, "channel"},
		{"version", func(feed *Feed) { feed.Latest.Version = "latest" }, "latest"},
		{"notes not https", func(feed *Feed) { feed.Latest.NotesURL = "http://example.com/notes" }, "notes_url"},
		{"no assets", func(feed *Feed) { feed.Latest.Assets = nil }, "no assets"},
		{"min_os", func(feed *Feed) { feed.Latest.MinOS["plan9"] = "1" }, "min_os"},
		{"expires before release", func(feed *Feed) { feed.Expires = feed.Latest.Date }, "expires"},
		{"unknown os", func(feed *Feed) { feed.Latest.Assets[0].OS = "plan9" }, "os"},
		{"unknown arch", func(feed *Feed) { feed.Latest.Assets[0].Arch = "riscv64" }, "arch"},
		{"unknown format", func(feed *Feed) { feed.Latest.Assets[0].Format = "dmg" }, "format"},
		{"asset not https", func(feed *Feed) { feed.Latest.Assets[0].URL = "http://example.com/a.zip" }, "https"},
		{"asset credentials", func(feed *Feed) { feed.Latest.Assets[0].URL = "https://user:pass@example.com/a.zip" }, "credentials"},
		{"zero size", func(feed *Feed) { feed.Latest.Assets[0].Size = 0 }, "size"},
		{"short digest", func(feed *Feed) { feed.Latest.Assets[0].SHA256 = "abc" }, "sha256"},
		{"bad digest", func(feed *Feed) { feed.Latest.Assets[0].SHA256 = strings.Repeat("g", 64) }, "sha256"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			feed := validFeed()
			test.mutate(&feed)
			err := feed.validate()
			if err == nil {
				t.Fatalf("validate accepted %s", test.name)
			}
			if !errors.Is(err, ErrManifest) {
				t.Errorf("error %v does not wrap ErrManifest", err)
			}
			if test.want != "" && !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %q does not mention %q", err, test.want)
			}
		})
	}
}

func TestFeedRefusesADuplicateAsset(t *testing.T) {
	feed := validFeed()
	feed.Latest.Assets = append(feed.Latest.Assets, feed.Latest.Assets[0])
	if err := feed.validate(); !errors.Is(err, ErrManifest) {
		t.Errorf("error %v does not wrap ErrManifest", err)
	}
}

func TestFeedPicksOneArchivePerPlatform(t *testing.T) {
	feed := validFeed()
	tests := []struct {
		goos, goarch, format string
	}{
		{"darwin", "arm64", FormatZIP},
		{"windows", "amd64", FormatNSIS},
		{"linux", "arm64", FormatTarGz},
	}
	for _, test := range tests {
		asset, err := feed.Asset(test.goos, test.goarch)
		if err != nil {
			t.Fatalf("Asset(%q, %q): %v", test.goos, test.goarch, err)
		}
		if asset.Format != test.format {
			t.Errorf("Asset(%q, %q) chose %q, want %q", test.goos, test.goarch, asset.Format, test.format)
		}
	}
	if _, err := feed.Asset("darwin", "amd64"); !errors.Is(err, ErrNoAsset) {
		t.Errorf("missing darwin/amd64: error %v does not wrap ErrNoAsset", err)
	}
}

func TestFeedRefusesAFormatTheOperatingSystemDoesNotInstall(t *testing.T) {
	tests := []struct {
		name string
		os   string
		set  func(*Feed)
	}{
		{"darwin nsis", "darwin", func(feed *Feed) { feed.Latest.Assets[0].Format = FormatNSIS }},
		{"windows tar", "windows", func(feed *Feed) { feed.Latest.Assets[1].Format = FormatTarGz }},
		{"linux zip", "linux", func(feed *Feed) { feed.Latest.Assets[3].Format = FormatZIP }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			feed := validFeed()
			test.set(&feed)
			if err := feed.validate(); !errors.Is(err, ErrManifest) {
				t.Errorf("error %v does not wrap ErrManifest", err)
			}
		})
	}
}

func TestAssetPrefersThePlatformInstaller(t *testing.T) {
	feed := Feed{Latest: Release{Assets: []Asset{
		{OS: "darwin", Arch: "arm64", Format: FormatNSIS},
		{OS: "darwin", Arch: "arm64", Format: FormatZIP},
	}}}
	asset, err := feed.Asset("darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Format != FormatZIP {
		t.Errorf("chose %q, want %q", asset.Format, FormatZIP)
	}
}

func TestFeedWithoutExpiryNeverExpires(t *testing.T) {
	feed := validFeed()
	feed.Expires = time.Time{}
	if err := feed.validate(); err != nil {
		t.Fatalf("a feed with no expires: %v", err)
	}
}
