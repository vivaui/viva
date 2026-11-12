package winpkg

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"time"
)

// zipTime is the timestamp of the entry, fixed so the same program always
// makes the same archive.
var zipTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// ZipProgram writes an archive holding the program under name and the given
// extra files under their destinations. The timestamp, mode and compression are
// fixed, and extras are ordered by destination, so the bytes depend only on the
// inputs.
func ZipProgram(program, name, archive string, extras ...ExtraFile) error {
	files, err := checkedExtraFiles(extras)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.Destination == name {
			return fmt.Errorf("winpkg: extra file destination %q collides with the program", name)
		}
	}
	out, err := os.Create(archive)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(out)
	err = writeZipFile(writer, name, program, 0o755)
	for _, file := range files {
		if err == nil {
			err = writeZipFile(writer, file.Destination, file.Source, 0o644)
		}
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(archive)
	}
	return err
}

// writeZipFile adds one ordinary file to the archive.
func writeZipFile(writer *zip.Writer, name, path string, mode os.FileMode) error {
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: zipTime}
	header.SetMode(mode)
	entry, err := writer.CreateHeader(header)
	if err == nil {
		_, err = io.Copy(entry, source)
	}
	if closeErr := source.Close(); err == nil {
		err = closeErr
	}
	return err
}
