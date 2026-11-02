package app

// MonospaceFamilies lists the fixed-pitch font families installed on this
// system, sorted and without duplicates. An application offers them to the user
// and gives the chosen one to ui.MonoFamily to draw terminal text in it.
//
// The list is read from the system on every call and takes a moment, since a
// family is measured rather than taken at its word, so it is for a settings
// page, not for painting. It is empty where the platform cannot enumerate fonts
// (a browser, an unsupported target), in which case ui.MonoSystem and
// ui.MonoClassic remain the choices.
func MonospaceFamilies() []string { return nativeMonospaceFamilies() }
