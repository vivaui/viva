package main

import (
	"bytes"
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
	"github.com/vivaui/viva/tools/patchgen"
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
	var validFor time.Duration
	var patchFroms []patchFrom
	flags.Func("patch-from", "make a delta update from this earlier release: os/arch=version=archive (repeatable; darwin and linux)", func(value string) error {
		parsed, err := parsePatchFrom(value)
		if err != nil {
			return err
		}
		patchFroms = append(patchFroms, parsed)
		return nil
	})
	flags.DurationVar(&validFor, "valid-for", 0, "how long clients trust the feed, such as 2160h; 0 (the default) never expires")
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
	if len(patchFroms) > 0 {
		if err := addPatches(assets, patchFroms, artifactsDir, baseURL, version, app, loaded.ExecutableName()); err != nil {
			return err
		}
	}
	key, err := readSigningKey(keyFile)
	if err != nil {
		return err
	}

	if validFor < 0 {
		return errors.New("-valid-for must not be negative; use 0 for a feed that never expires")
	}
	now := time.Now().UTC().Truncate(time.Second)
	feed := update.Feed{
		Schema: update.SchemaVersion, App: app, Channel: channel,
		Latest: update.Release{
			Version: version, Date: now, NotesURL: notes, Assets: assets,
		},
	}
	if validFor > 0 {
		feed.Expires = now.Add(validFor)
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
	if !feed.Expires.IsZero() {
		fmt.Fprintf(output, "  expires  %s\n", feed.Expires.Format(time.RFC3339))
	}
	fmt.Fprintf(output, "  assets   %d\n", len(assets))
	patches := 0
	for _, asset := range assets {
		patches += len(asset.Patches)
	}
	if patches > 0 {
		fmt.Fprintf(output, "  patches  %d\n", patches)
	}
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

// patchFrom is one --patch-from value: a previous release to diff against.
type patchFrom struct {
	os, arch, version, archive string
}

func parsePatchFrom(value string) (patchFrom, error) {
	parts := strings.SplitN(value, "=", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return patchFrom{}, errors.New("--patch-from wants os/arch=version=archive, such as darwin/arm64=1.0.0=dist/notes-1.0.0-darwin-arm64.zip")
	}
	target, version, archive := parts[0], parts[1], parts[2]
	goos, goarch, ok := strings.Cut(target, "/")
	if !ok || (goos != "darwin" && goos != "linux") {
		return patchFrom{}, fmt.Errorf("--patch-from %q: patches are made for darwin and linux", target)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return patchFrom{}, fmt.Errorf("--patch-from %q: architecture must be amd64 or arm64", target)
	}
	return patchFrom{os: goos, arch: goarch, version: version, archive: archive}, nil
}

// addPatches writes a patch for every --patch-from that names a listed asset
// and adds it to the asset. Every argument must match an asset, so a typo in
// the platform cannot publish a feed that silently lacks its delta.
func addPatches(assets []update.Asset, froms []patchFrom, dir, baseURL, version, app, executable string) error {
	base := strings.TrimRight(baseURL, "/")
	byTarget := make(map[string][]patchFrom, len(froms))
	for _, from := range froms {
		key := from.os + "/" + from.arch
		for _, existing := range byTarget[key] {
			if existing.version == from.version {
				return fmt.Errorf("feed: --patch-from %s=%s is given twice", key, from.version)
			}
		}
		if cmp, err := update.Compare(from.version, version); err != nil {
			return fmt.Errorf("feed: --patch-from %s=%s: %v", key, from.version, err)
		} else if cmp >= 0 {
			return fmt.Errorf("feed: --patch-from %s=%s must be older than the release %s", key, from.version, version)
		}
		if _, err := os.Stat(from.archive); err != nil {
			return fmt.Errorf("feed: --patch-from %s=%s: %w", key, from.version, err)
		}
		byTarget[key] = append(byTarget[key], from)
	}
	matched := make(map[string]int, len(byTarget))
	for i := range assets {
		asset := &assets[i]
		key := asset.OS + "/" + asset.Arch
		froms := byTarget[key]
		if len(froms) == 0 {
			continue
		}
		name, err := url.PathUnescape(strings.TrimPrefix(asset.URL, base+"/"))
		if err != nil {
			return fmt.Errorf("feed: %s: %w", asset.URL, err)
		}
		newArchive := filepath.Join(dir, name)
		for _, from := range froms {
			var encoded bytes.Buffer
			if _, err := patchgen.Write(patchgen.Request{
				App: app, FromVersion: from.version, ToVersion: version,
				OS: asset.OS, Arch: asset.Arch,
				OldArchive: from.archive, NewArchive: newArchive,
				Executable: executable,
				Output:     &encoded,
			}); err != nil {
				return fmt.Errorf("feed: patch %s from %s: %w", key, from.version, err)
			}
			patchName := name + "." + from.version + ".patch"
			if err := writePatchFileBeside(filepath.Join(dir, patchName), encoded.Bytes()); err != nil {
				return err
			}
			sum := sha256.Sum256(encoded.Bytes())
			asset.Patches = append(asset.Patches, update.Patch{
				FromVersion: from.version,
				URL:         base + "/" + url.PathEscape(patchName),
				Size:        int64(encoded.Len()),
				SHA256:      hex.EncodeToString(sum[:]),
			})
			matched[key]++
		}
		sort.Slice(asset.Patches, func(a, b int) bool {
			order, err := update.Compare(asset.Patches[a].FromVersion, asset.Patches[b].FromVersion)
			return err == nil && order < 0
		})
	}
	for key, froms := range byTarget {
		if matched[key] != len(froms) {
			return fmt.Errorf("feed: --patch-from %s names no %s/%s archive in %s", key, strings.Split(key, "/")[0], strings.Split(key, "/")[1], displayPath(dir))
		}
	}
	return nil
}

// writeFeedFile writes a patch beside the archives, named only when complete.
func writePatchFileBeside(name string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".viva-feed-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(temporary)
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, name); err != nil {
		os.Remove(temporary)
		return err
	}
	return nil
}
