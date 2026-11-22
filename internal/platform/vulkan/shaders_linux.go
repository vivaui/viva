package vulkan

import _ "embed"

//go:embed shaders/scene.vert.spv
var sceneVertexShader []byte

//go:embed shaders/scene.frag.spv
var sceneFragmentShader []byte

//go:embed shaders/text.vert.spv
var textVertexShader []byte

//go:embed shaders/text.frag.spv
var textFragmentShader []byte

//go:embed shaders/text_lcd.frag.spv
var textLCDFragmentShader []byte
