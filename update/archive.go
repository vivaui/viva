package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxExtractedBytes bounds what one archive may unpack to, so a small download
// cannot fill the disk.
const maxExtractedBytes int64 = 2 << 30

// extractZip unpacks archive into destination, refusing entries that would
// write outside it, links, and unusual file types. File modes are kept (less
// setuid, setgid and sticky), so an application's executable stays executable.
func extractZip(archive, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("update: reading %s: %w", filepath.Base(archive), err)
	}
	defer reader.Close()
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	seen := make(map[string]bool, len(reader.File))
	var total int64
	for _, entry := range reader.File {
		if entry.Name == "" {
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s is a link", ErrUnsafeArchive, entry.Name)
		}
		target, err := archivePath(destination, entry.Name)
		if err != nil {
			return err
		}
		if seen[target] {
			return fmt.Errorf("%w: %s appears twice", ErrUnsafeArchive, entry.Name)
		}
		seen[target] = true
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("update: %w", err)
			}
			continue
		}
		if !entry.Mode().IsRegular() {
			return fmt.Errorf("%w: %s is not a regular file", ErrUnsafeArchive, entry.Name)
		}
		if err := writeArchiveFile(target, entry.Mode(), func() (io.ReadCloser, error) {
			return entry.Open()
		}, &total); err != nil {
			return err
		}
	}
	return nil
}

// extractTarGz unpacks a gzipped tar into destination with the same refusals as
// extractZip.
func extractTarGz(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("update: reading %s: %w", filepath.Base(archive), err)
	}
	defer compressed.Close()
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	reader := tar.NewReader(compressed)
	seen := make(map[string]bool)
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("update: reading %s: %w", filepath.Base(archive), err)
		}
		if header.Name == "" || header.Name == "./" {
			continue
		}
		target, err := archivePath(destination, header.Name)
		if err != nil {
			return err
		}
		if seen[target] {
			return fmt.Errorf("%w: %s appears twice", ErrUnsafeArchive, header.Name)
		}
		seen[target] = true
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("update: %w", err)
			}
		case tar.TypeReg:
			if err := writeArchiveFile(target, os.FileMode(header.Mode), func() (io.ReadCloser, error) {
				return io.NopCloser(reader), nil
			}, &total); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %s is not a regular file or directory", ErrUnsafeArchive, header.Name)
		}
	}
	return nil
}

func archivePath(destination, name string) (string, error) {
	relative := filepath.FromSlash(name)
	// A name rooted at / or \ or carrying a drive letter is absolute on some
	// platform, even when filepath.IsAbs says no on this one (Windows does not
	// call "/tmp/x" absolute). The archive is the same file everywhere, so it is
	// refused everywhere.
	if filepath.IsAbs(relative) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) || filepath.VolumeName(relative) != "" {
		return "", fmt.Errorf("%w: %s is absolute", ErrUnsafeArchive, name)
	}
	clean := filepath.Clean(relative)
	if clean == "." {
		return destination, nil
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s leaves the archive", ErrUnsafeArchive, name)
	}
	target := filepath.Join(destination, clean)
	inside, err := filepath.Rel(destination, target)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s leaves the archive", ErrUnsafeArchive, name)
	}
	return target, nil
}

func writeArchiveFile(target string, mode os.FileMode, open func() (io.ReadCloser, error), total *int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	source, err := open()
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	defer source.Close()
	permissions := mode.Perm() & 0o777
	if permissions == 0 {
		permissions = 0o644
	}
	destination, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, permissions)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, maxExtractedBytes-*total+1))
	closeErr := destination.Close()
	*total += written
	if copyErr != nil {
		return fmt.Errorf("update: unpacking %s: %w", filepath.Base(target), copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("update: %w", closeErr)
	}
	if *total > maxExtractedBytes {
		return fmt.Errorf("%w: %s unpacks to more than %d bytes", ErrUnsafeArchive, filepath.Base(target), maxExtractedBytes)
	}
	return nil
}
