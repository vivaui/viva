package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/vivaui/viva/tools/manifest"
	"github.com/vivaui/viva/update"
)

// runFeed writes the signed update feed of the archives viva package made. It
// names every archive by the platform, architecture and format in its file
// name, so the feed and the files it lists cannot disagree about what a
// download is.
func runFeed(arguments []string, output, _ io.Writer) error {
	c, _ := findCommand("feed")
	flags := flag.NewFlagSet("feed", flag.ContinueOnError)
	var moduleDir, artifactsDir, out, version, app, channel, baseURL, notes, keyFile string
	flags.StringVar(&moduleDir, "C", ".", "directory holding viva.toml")
	flags.StringVar(&artifactsDir, "dir", "", "directory holding the packaged archives (default [build] out)")
	flags.StringVar(&out, "out", "", "feed file to write (default <dir>/latest.json)")
	flags.StringVar(&version, "version", "", "release version (default [app] version)")
	flags.StringVar(&app, "app", "", "application identifier (default [app] id)")
	flags.StringVar(&channel, "channel", "stable", "release channel")
	flags.StringVar(&baseURL, "base-url", "", "HTTPS address the archives are served from (required)")
	flags.StringVar(&notes, "notes", "", "HTTPS page describing the release")
	validFor := 90 * 24 * time.Hour
	flags.DurationVar(&validFor, "valid-for", validFor, "how long clients trust the feed; publish a new one before it runs out")
	flags.StringVar(&keyFile, "key", "", "Ed25519 private key file (default $VIVA_UPDATE_KEY)")
	positional, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(positional) != 0 {
		return errors.New("feed takes no arguments; run 'viva help feed'")
	}
	if baseURL == "" {
		return errors.New("feed needs -base-url: the HTTPS address the archives are served from, such as https://example.com/app")
	}
	if err := checkHTTPSFlag("-base-url", baseURL); err != nil {
		return err
	}
	if notes != "" {
		if err := checkHTTPSFlag("-notes", notes); err != nil {
			return err
		}
	}
	if strings.ContainsAny(channel, " \t\r\n") {
		return fmt.Errorf("-channel %q contains whitespace; a channel is one word, such as stable or beta", channel)
	}

	loaded, err := manifest.Load(moduleDir)
	if err != nil {
		return err
	}
	if app == "" {
		app = loaded.App.ID
		if app == "" {
			return fmt.Errorf("feed needs -app or [app] id in %s", manifest.FileName)
		}
	}
	if version == "" {
		version = loaded.App.Version
		if version == "" {
			return fmt.Errorf("feed needs -version or [app] version in %s", manifest.FileName)
		}
	}
	// The update client reads versions with update.Compare, which wants
	// major.minor.patch; a feed it cannot order would be refused long after it
	// was published.
	if _, err := update.Compare(version, version); err != nil {
		return fmt.Errorf("version %q cannot go in an update feed (set [app] version or -version to major.minor.patch, such as 1.4.0): %w", version, err)
	}

	if artifactsDir == "" {
		artifactsDir = loaded.Path(loaded.Build.Out)
	}
	assets, err := scanArtifacts(artifactsDir, baseURL, version)
	if err != nil {
		return err
	}
	key, err := readSigningKey(keyFile)
	if err != nil {
		return err
	}

	if validFor <= 0 {
		return errors.New("-valid-for must be positive, such as 2160h (90 days)")
	}
	now := time.Now().UTC().Truncate(time.Second)
	feed := update.Feed{
		Schema: update.SchemaVersion, App: app, Channel: channel,
		Expires: now.Add(validFor),
		Latest: update.Release{
			Version: version, Date: now, NotesURL: notes, Assets: assets,
		},
	}
	data, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		return err
	}
	// The signature covers exactly the bytes written below, never a re-encoding
	// of them.
	signature, err := update.Sign(data, key)
	if err != nil {
		return err
	}
	if out == "" {
		out = filepath.Join(artifactsDir, "latest.json")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := writeFeedOutput(out, data); err != nil {
		return fmt.Errorf("writing the feed %s: %w", displayPath(out), err)
	}
	signaturePath := out + ".sig"
	if err := writeFeedOutput(signaturePath, []byte(signature)); err != nil {
		return fmt.Errorf("writing the signature %s: %w", displayPath(signaturePath), err)
	}

	fmt.Fprintf(output, "wrote %s\n", displayPath(out))
	fmt.Fprintf(output, "  app      %s\n", app)
	fmt.Fprintf(output, "  version  %s\n", version)
	fmt.Fprintf(output, "  channel  %s\n", channel)
	fmt.Fprintf(output, "  expires  %s\n", feed.Expires.Format(time.RFC3339))
	fmt.Fprintf(output, "  assets   %d\n", len(assets))
	fmt.Fprintf(output, "  signed   %s\n", displayPath(signaturePath))
	return nil
}

// writeFeedOutput writes one feed file and gives it mode 0644 whatever the
// umask is, as the packaging code does for its archives.
func writeFeedOutput(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// checkHTTPSFlag refuses a URL a feed client would not accept: one that is not
// HTTPS, has no host, or carries credentials that would leak into the feed.
func checkHTTPSFlag(name, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s %q is not a URL: %w", name, raw, err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s %q must be an HTTPS URL, such as https://example.com/app", name, raw)
	}
	if parsed.User != nil {
		return fmt.Errorf("%s %q must not contain credentials", name, raw)
	}
	return nil
}

// scanArtifacts lists the archives in dir and describes each one for the feed.
// baseURL is where they are served from; a trailing slash is optional.
func scanArtifacts(dir, baseURL, version string) ([]update.Asset, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("archive directory %s does not exist; run 'viva package' first or pass -dir", displayPath(dir))
		}
		return nil, fmt.Errorf("reading the archive directory %s: %w", displayPath(dir), err)
	}
	base := strings.TrimRight(baseURL, "/")
	seen := make(map[string]string)
	var assets []update.Asset
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		name := entry.Name()
		goos, goarch, format, artifactVersion, ok := classifyArtifact(name)
		if !ok {
			continue
		}
		if artifactVersion != version {
			return nil, fmt.Errorf("%s is version %s, but the feed is %s; package the version the feed names (or pass the matching -version)", name, artifactVersion, version)
		}
		key := goos + "/" + goarch + "/" + format
		if previous, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%s and %s are both the %s archive; a feed lists one file per platform, architecture and format", previous, name, key)
		}
		seen[key] = name
		size, digest, err := fileDigest(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		assets = append(assets, update.Asset{
			OS: goos, Arch: goarch, Format: format,
			URL: base + "/" + url.PathEscape(name), Size: size, SHA256: digest,
		})
	}
	if len(assets) == 0 {
		return nil, fmt.Errorf("no update archives found in %s; viva package writes *-darwin-<arch>.zip, *-windows-<arch>-setup.exe, *-windows-<arch>.zip and *-linux-<arch>.tar.gz", displayPath(dir))
	}
	sort.Slice(assets, func(i, j int) bool {
		left := assets[i].OS + "/" + assets[i].Arch + "/" + assets[i].Format
		right := assets[j].OS + "/" + assets[j].Arch + "/" + assets[j].Format
		return left < right
	})
	return assets, nil
}

// artifactName matches the names viva package writes:
// <program>-<version>-<os>-<arch>[-setup].<zip|tar.gz|exe>. The version is
// captured so a feed cannot name one version and list an archive of another.
var artifactName = regexp.MustCompile(`-(v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)*)-(darwin|windows|linux)-(amd64|arm64)(-setup)?\.(zip|tar\.gz|exe)$`)

// classifyArtifact names the platform, architecture, format and version of a
// file from the name viva package gives it. Any other file is ignored, which
// leaves notes, signatures and the feed itself out of the feed.
func classifyArtifact(name string) (goos, goarch, format, version string, ok bool) {
	match := artifactName.FindStringSubmatch(name)
	if match == nil {
		return "", "", "", "", false
	}
	version, goos, goarch, setup, extension := match[1], match[2], match[3], match[4], match[5]
	switch {
	case goos == "darwin" && setup == "" && extension == "zip":
		format = update.FormatZIP
	case goos == "windows" && setup == "-setup" && extension == "exe":
		format = update.FormatNSIS
	case goos == "windows" && setup == "" && extension == "zip":
		format = update.FormatZIP
	case goos == "linux" && setup == "" && extension == "tar.gz":
		format = update.FormatTarGz
	default:
		return "", "", "", "", false
	}
	return goos, goarch, format, version, true
}

// fileDigest returns the length and lowercase SHA-256 digest of one archive.
func fileDigest(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", fmt.Errorf("reading %s: %w", displayPath(path), err)
	}
	hash := sha256.New()
	size, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return 0, "", fmt.Errorf("reading %s: %w", displayPath(path), copyErr)
	}
	if closeErr != nil {
		return 0, "", fmt.Errorf("reading %s: %w", displayPath(path), closeErr)
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

// readSigningKey reads the Ed25519 private key from path, or from
// $VIVA_UPDATE_KEY when path is empty.
func readSigningKey(path string) (ed25519.PrivateKey, error) {
	if path == "" {
		text := os.Getenv("VIVA_UPDATE_KEY")
		if strings.TrimSpace(text) == "" {
			return nil, errors.New("feed needs a signing key: pass -key FILE or set VIVA_UPDATE_KEY (create one with 'viva update-keygen')")
		}
		key, err := update.ParsePrivateKey(text)
		if err != nil {
			return nil, fmt.Errorf("invalid VIVA_UPDATE_KEY: %w (it must be the base64 64-byte private key 'viva update-keygen' writes)", err)
		}
		return key, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the signing key %s: %w", displayPath(path), err)
	}
	key, err := update.ParsePrivateKey(string(data))
	if err != nil {
		return nil, fmt.Errorf("invalid signing key %s: %w (it must be the base64 64-byte private key 'viva update-keygen' writes)", displayPath(path), err)
	}
	return key, nil
}

// runUpdateKeygen writes the Ed25519 key pair viva feed signs with: the
// private key in the 64-byte form update.ParsePrivateKey reads, and the public
// key in the 32-byte form an application trusts.
func runUpdateKeygen(arguments []string, output, _ io.Writer) error {
	c, _ := findCommand("update-keygen")
	flags := flag.NewFlagSet("update-keygen", flag.ContinueOnError)
	privatePath := "update.key"
	publicPath := ""
	force := false
	flags.StringVar(&privatePath, "private", privatePath, "private key file to write (base64 of 64 bytes)")
	flags.StringVar(&publicPath, "public", publicPath, "public key file to write (base64 of 32 bytes); empty writes none")
	flags.BoolVar(&force, "force", force, "replace key files that already exist")
	positional, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(positional) != 0 {
		return errors.New("update-keygen takes no arguments; run 'viva help update-keygen'")
	}
	if privatePath == "" {
		return errors.New("update-keygen needs -private FILE to write the private key to")
	}
	if publicPath != "" && filepath.Clean(publicPath) == filepath.Clean(privatePath) {
		return errors.New("-public and -private name the same file; the public key needs a file of its own")
	}
	if !force {
		for _, path := range []string{privatePath, publicPath} {
			if path == "" {
				continue
			}
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists; use -force to replace it", displayPath(path))
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	privateText := base64.StdEncoding.EncodeToString(private)
	if err := writeKeyFile(privatePath, privateText, 0o600, force); err != nil {
		return err
	}
	publicText := base64.StdEncoding.EncodeToString(public)
	if publicPath != "" {
		if err := writeKeyFile(publicPath, publicText, 0o644, force); err != nil {
			return err
		}
	}

	fmt.Fprintln(output, "generated update key")
	fmt.Fprintln(output, "  warning  keep update.key private; never commit it")
	fmt.Fprintf(output, "  private  %s\n", displayPath(privatePath))
	if publicPath != "" {
		fmt.Fprintf(output, "  file     %s\n", displayPath(publicPath))
	}
	fmt.Fprintf(output, "  public   %s\n", publicText)
	return nil
}

// writeKeyFile writes one base64 key and a final newline, giving it mode and
// refusing to replace a file unless force is set.
func writeKeyFile(path, text string, mode os.FileMode, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, mode)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; use -force to replace it", displayPath(path))
		}
		return err
	}
	// Chmod sets the mode even when the file already existed under -force and
	// was made by another umask.
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := io.WriteString(file, text+"\n"); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
