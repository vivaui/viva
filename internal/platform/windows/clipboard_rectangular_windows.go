package windows

func SetClipboardRectangularText(value string) error {
	return SetClipboardText(value)
}

func ClipboardTextWithFormat() (string, bool, error) {
	value, err := ClipboardText()
	return value, false, err
}
