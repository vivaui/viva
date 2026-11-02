//go:build windows

package windows

import (
	"reflect"
	"testing"
)

func TestOpenWithCommandPassesOnePathAndHidesItsConsole(t *testing.T) {
	command := openWithCommand(`C:\Program Files\Editor\Editor.exe`, `C:\Notes\server config.yml`)
	if want := []string{`C:\Program Files\Editor\Editor.exe`, `C:\Notes\server config.yml`}; !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("command args = %q, want %q", command.Args, want)
	}
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow ||
		command.SysProcAttr.CreationFlags&openWithCreateNoWindow == 0 {
		t.Fatal("the custom editor command does not hide its console window")
	}
}
