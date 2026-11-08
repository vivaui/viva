//go:build js && wasm

package app

import (
	"errors"
	"fmt"

	"github.com/vivaui/viva/internal/platform"
	webplatform "github.com/vivaui/viva/internal/platform/web"
	"github.com/vivaui/viva/ui"
)

func asyncOpenFiles(options OpenFileOptions, done func([]platform.FileRef, error)) error {
	return webplatform.OpenFilesAsync(platform.OpenFilesOptions{
		Title:         options.Title,
		Filters:       platformFilters(options.Filters),
		AllowMultiple: options.AllowMultiple,
	}, done)
}

func asyncSaveFile(options SaveFileOptions, done func(platform.FileSink, error)) error {
	return webplatform.SaveFileAsync(platform.SaveFileOptions{
		Title:            options.Title,
		SuggestedName:    options.SuggestedName,
		DefaultExtension: options.DefaultExtension,
		Filters:          platformFilters(options.Filters),
	}, done)
}

func platformFilters(filters []FileFilter) []platform.FileFilter {
	converted := make([]platform.FileFilter, len(filters))
	for index, filter := range filters {
		converted[index] = platform.FileFilter{Name: filter.Name, Pattern: filter.Pattern}
	}
	return converted
}

func asyncFileError(action string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, platform.ErrCanceled):
		return ErrCanceled
	case errors.Is(err, webplatform.ErrPermissionDenied):
		return fmt.Errorf("%s: %w: %v", action, ErrPermissionDenied, err)
	case errors.Is(err, webplatform.ErrUserGestureRequired):
		return fmt.Errorf("%s: %w: %v", action, ErrUserGestureRequired, err)
	case errors.Is(err, webplatform.ErrUnsupported), errors.Is(err, webplatform.ErrUnavailable):
		return fmt.Errorf("%s: %w: %v", action, ErrUnsupported, err)
	default:
		return fmt.Errorf("app: %s: %w", action, err)
	}
}

// The canvas painter is shared with the native build, but the browser backend
// never calls its file drop path; the web runtime supplies Files itself.
func fileDropFilesFromPaths([]string) []ui.FileDropFile { return nil }
