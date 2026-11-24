package app

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

type decorationTestControl struct {
	WindowControl
	decorated bool
	done      func(error)
	err       error
}

func (c *decorationTestControl) SetDecorated(decorated bool, done func(error)) error {
	c.decorated, c.done = decorated, done
	return c.err
}

func TestSetDecoratedReportsCompletionOnlyFromHost(t *testing.T) {
	control := &decorationTestControl{decorated: true}
	window := &Window{control: control}
	var completed []error
	if err := window.SetDecorated(false, func(err error) { completed = append(completed, err) }); err != nil {
		t.Fatal(err)
	}
	if control.decorated || control.done == nil || len(completed) != 0 {
		t.Fatal("submission did not reach the host, or completed before the host replied")
	}
	rollback := errors.New("previous frame restored")
	control.done(rollback)
	if len(completed) != 1 || !errors.Is(completed[0], rollback) {
		t.Fatalf("completion = %v", completed)
	}
}

func TestSetDecoratedRejectsUnsupportedAndClosedWindows(t *testing.T) {
	unsupported := &Window{control: struct{ WindowControl }{}}
	for _, window := range []*Window{unsupported, {control: &decorationTestControl{err: platform.ErrWindowDecorationUnsupported}}} {
		if err := window.SetDecorated(false, func(error) { t.Error("unsupported submission completed") }); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("unsupported submission: %v", err)
		}
	}
	control := &decorationTestControl{}
	window := &Window{control: control}
	if err := window.SetDecorated(false, nil); err == nil {
		t.Fatal("submission without a completion callback succeeded")
	}
	window.closed.Store(true)
	if err := window.SetDecorated(false, func(error) {}); err == nil || control.done != nil {
		t.Fatalf("closed submission reached host: %v", err)
	}
}
