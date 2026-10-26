//go:build darwin

package app

func setReopenHandler(config *Config, window *Window) {
	config.onReopen = window.Show
}
