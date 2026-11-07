// Package interaction holds the platform-independent behavior shared by Viva's
// native and browser painters: how painted hits become routing regions, which
// target owns focus, how presses capture a target, and how key releases pair
// with the presses the application saw.
//
// Painting, text shaping, native views, renderer publishing, and browser DOM
// services stay in their own platform packages. This package depends only on
// ui and internal/input so both painters can use the same rules.
package interaction
