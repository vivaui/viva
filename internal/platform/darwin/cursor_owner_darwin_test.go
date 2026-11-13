//go:build darwin

package darwin

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// 用真实 NSView 命中测试验证所有权交接，避免用 Go 替身重复一份平台判断。
func TestNativeWebCursorOwnership(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "cursor-owner")
	compile := exec.Command("xcrun", "clang", "-x", "objective-c", "-fno-objc-arc", "-framework", "AppKit", "-I", ".", "testdata/cursor_owner.m", "-o", binary)
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("编译原生光标命中回归失败：%v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("原生光标所有权回归失败：%v\n%s", err, output)
	}
}
