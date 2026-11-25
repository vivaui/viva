package platform

// WebDevTools is the part of a WebView that can open the engine's developer tools. It is optional:
// a platform whose pages do not implement it has no developer tools, and app.WebView reports
// ErrUnsupported. ShowDevTools runs on the event-loop thread; an error wrapping
// ErrWebViewUnsupported says the engine cannot open them from the application.
type WebDevTools interface {
	ShowDevTools() error
}
