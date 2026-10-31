package linux

import "fmt"

// refreshText rasterizes every label drawn from text again, for a change in how the desktop
// draws text (font_settings.go). The window presents afterwards.
func (renderer *vulkanRenderer) refreshText() error {
	if renderer.device == 0 || renderer.commandPool == 0 {
		return nil
	}
	if err := vkCheck(renderer.api.deviceWaitIdle(renderer.device), "wait before text refresh"); err != nil {
		return err
	}
	if err := renderer.resetCommands("reset commands before text refresh"); err != nil {
		return fmt.Errorf("linux: refresh text: %w", err)
	}
	// The device is idle, so the images the labels are drawn from can go; uploading rasterizes
	// every label that has none.
	for index := range renderer.text.labels {
		label := &renderer.text.labels[index]
		if len(label.label.Bitmap) == 0 {
			label.closeImage(renderer)
		}
	}
	if err := renderer.text.upload(renderer, renderer.pixelScale()); err != nil {
		return fmt.Errorf("linux: refresh text: %w", err)
	}
	return renderer.recordCommands(vkExtent2D{Width: renderer.width, Height: renderer.height})
}
