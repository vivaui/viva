//go:build darwin && !ios

package darwin

import _ "embed"

//go:embed shaders/scene.metallib
var sceneShaderLibrary []byte
