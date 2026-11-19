//go:build js && wasm

package web

import (
	"embed"
	"fmt"
	"syscall/js"
)

//go:embed shaders/scene.vert.glsl shaders/scene.frag.glsl shaders/text.vert.glsl shaders/text.frag.glsl
var shaderFiles embed.FS

func shaderSource(name string) (string, error) {
	source, err := shaderFiles.ReadFile("shaders/" + name)
	if err != nil {
		return "", err
	}
	return string(source), nil
}

func compileShader(gl js.Value, kind string, source string) (js.Value, error) {
	shader := gl.Call("createShader", gl.Get(kind))
	gl.Call("shaderSource", shader, source)
	gl.Call("compileShader", shader)
	if !gl.Call("getShaderParameter", shader, gl.Get("COMPILE_STATUS")).Bool() {
		log := gl.Call("getShaderInfoLog", shader).String()
		gl.Call("deleteShader", shader)
		return js.Undefined(), fmt.Errorf("webgl2: %s shader: %s", kind, log)
	}
	return shader, nil
}

func linkProgram(gl js.Value, vertex, fragment js.Value) (js.Value, error) {
	program := gl.Call("createProgram")
	gl.Call("attachShader", program, vertex)
	gl.Call("attachShader", program, fragment)
	gl.Call("linkProgram", program)
	if !gl.Call("getProgramParameter", program, gl.Get("LINK_STATUS")).Bool() {
		log := gl.Call("getProgramInfoLog", program).String()
		gl.Call("deleteProgram", program)
		return js.Undefined(), fmt.Errorf("webgl2: program: %s", log)
	}
	return program, nil
}
