//go:build !darwin && !windows && !linux

package app

func driverClipboardText() (string, error, bool) { return "", nil, false }

func driverSetClipboardText(string) (error, bool) { return nil, false }

func driverSetPrimarySelection(string) (error, bool) { return nil, false }

func driverPrimarySelection() (string, error, bool) { return "", nil, false }

func driverClipboardImage() (png []byte, err error, handled bool) { return nil, nil, false }

func driverOpenURL(string) (error, bool)  { return nil, false }
func driverOpenPath(string) (error, bool) { return nil, false }
func driverOpenWith(string, string) (error, bool) {
	return nil, false
}

func driverOpenFiles(OpenFileOptions) ([]string, error, bool) {
	return nil, nil, false
}

func driverOpenDirectory(OpenDirectoryOptions) (string, error, bool) {
	return "", nil, false
}

func driverSaveFile(SaveFileOptions) (string, error, bool) {
	return "", nil, false
}

func driverStoreSecret(string, string, []byte) (error, bool) { return nil, false }

func driverLoadSecret(string, string) ([]byte, error, bool) { return nil, nil, false }

func driverDeleteSecret(string, string) (error, bool) { return nil, false }
