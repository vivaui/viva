//go:build linux

package proxy

import "net/url"

// nativeSystemProxy is the environment, which SystemProxy has already read:
// Linux desktops that keep a proxy in GSettings do not export it, and asking
// them would mean starting a session-bus or GIO connection for a setting most
// programs do not read. A system proxy configured there is not reported.
func nativeSystemProxy(*url.URL) (*url.URL, error) { return nil, nil }
