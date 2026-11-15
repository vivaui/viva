//go:build js && wasm

package proxy

import "net/url"

func nativeSystemProxy(*url.URL) (*url.URL, error) { return nil, ErrUnavailable }
