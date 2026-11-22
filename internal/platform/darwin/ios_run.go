//go:build ios

package darwin

/*
#include "native.h"
#include "ios_host.h"
*/
import "C"

import (
	"fmt"
	"os"
	"runtime"

	"github.com/vivaui/viva/internal/platform"
)

// iosLaunch is what the application started with. UIKit owns the process from
// UIApplicationMain on, and has the application launched before anything of ours
// may make a window, so start runs from the launch callback, not before.
var iosLaunch struct {
	start platform.ApplicationStart
}

// Run enters the UIKit event loop with one window.
func Run(config Config) error {
	return RunApplication(func(application platform.Application) error {
		_, err := application.OpenWindow(config)
		return err
	})
}

// RunApplication enters the UIKit event loop; start opens the windows once the
// application has launched. It returns only if UIKit fails to start: an iOS
// application is ended by the system.
func RunApplication(start platform.ApplicationStart) error {
	runtime.LockOSThread()
	iosLaunch.start = start
	return nativeStatusError(C.viva_ios_main())
}

// vivaIOSLaunched runs on the main thread, in application:didFinishLaunching.
//
//export vivaIOSLaunched
func vivaIOSLaunched() {
	if err := iosLaunch.start(&nativeApplication{}); err != nil {
		// There is no caller to return to: UIKit runs the process.
		fmt.Fprintf(os.Stderr, "viva: %v\n", err)
		C.viva_ios_terminate(1)
	}
}
