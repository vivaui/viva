//go:build darwin && !ios

package app

func setReopenHandler(config *Config, window *Window) {
	config.onReopen = window.Show
}
