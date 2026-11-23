//go:build darwin && !ios

package darwin

/*
#include "native.h"
*/
import "C"

import "github.com/vivaui/viva/internal/platform"

// Run enters the Cocoa event loop with one window.
func Run(config Config) error {
	return RunApplication(func(application platform.Application) error {
		_, err := application.OpenWindow(config)
		return err
	})
}

// RunApplication enters one Cocoa event loop for every window opened by start.
func RunApplication(start platform.ApplicationStart) error {
	if status := C.viva_application_begin(); status != 0 {
		return nativeStatusError(status)
	}
	application := &nativeApplication{}
	defer application.SetQuitRequestHandler(nil)
	defer application.SetReopenHandler(nil)
	if err := start(application); err != nil {
		C.viva_application_close_all()
		if application.windows > 0 {
			_ = C.viva_application_run()
		}
		return err
	}
	status := C.viva_application_run()
	if application.firstErr != nil {
		return application.firstErr
	}
	return nativeStatusError(status)
}
