//go:build !(js && wasm)

package app

func asyncRegisterFont(data []byte, done func(string, func(), error)) error {
	family, unregister, err := RegisterFont(data)
	done(family, unregister, err)
	return nil
}
