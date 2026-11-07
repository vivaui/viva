package app

import "fmt"

// RegisterFontAsync is RegisterFont with a completion callback, the way to
// register a font on the Web target, where FontFace.load settles through a
// Promise. On native targets done runs before the call returns, with the same
// result as RegisterFont. done runs exactly once.
//
// The browser receives the family from the font's own name table, so the same
// family's regular, italic and bold files combine as they do natively. The
// unregister function removes this registration; registering the same bytes
// again shares it.
func RegisterFontAsync(data []byte, done func(family string, unregister func(), err error)) error {
	if done == nil {
		return fmt.Errorf("app: nil font callback")
	}
	if len(data) == 0 {
		return fmt.Errorf("app: font data is empty")
	}
	return asyncRegisterFont(data, done)
}
