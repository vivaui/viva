//go:build darwin || windows || linux

package app

import (
	"testing"
)

func TestDriverInterceptsPreventSleep(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	release, err := PreventSleep("Motty: transferring files")
	if err != nil {
		t.Fatal(err)
	}
	if reasons := driver.AwakeReasons(); len(reasons) != 1 || reasons[0] != "Motty: transferring files" {
		t.Fatalf("AwakeReasons = %v", reasons)
	}
	second, err := PreventSleep("Motty: remote desktop")
	if err != nil {
		t.Fatal(err)
	}
	if reasons := driver.AwakeReasons(); len(reasons) != 2 {
		t.Fatalf("AwakeReasons = %v", reasons)
	}
	release()
	release() // releasing twice does not end the other request
	if reasons := driver.AwakeReasons(); len(reasons) != 1 || reasons[0] != "Motty: remote desktop" {
		t.Fatalf("AwakeReasons = %v after one release", reasons)
	}
	second()
	if reasons := driver.AwakeReasons(); len(reasons) != 0 {
		t.Fatalf("AwakeReasons = %v after both releases", reasons)
	}
}
