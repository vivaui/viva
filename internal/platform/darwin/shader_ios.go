//go:build ios

package darwin

/*
#include "ios_host.h"
*/
import "C"

import _ "embed"

// A Metal library records the platform it was built for, and the simulator's is
// not the device's, so both are shipped and the one the build runs on is chosen.
// tools/generate_darwin_shaders.sh makes them.
var (
	//go:embed shaders/scene_ios.metallib
	sceneShaderLibraryDevice []byte
	//go:embed shaders/scene_ios_simulator.metallib
	sceneShaderLibrarySimulator []byte
)

// sceneShaderLibrary is the library of the platform the program runs on.
var sceneShaderLibrary = func() []byte {
	if C.viva_ios_is_simulator() != 0 {
		return sceneShaderLibrarySimulator
	}
	return sceneShaderLibraryDevice
}()
