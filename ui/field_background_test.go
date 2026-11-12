package ui_test

import (
	"context"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestFieldBackgroundSnapshotSurvivesTypingAndDropsStaleResult(t *testing.T) {
	const initial = "旧草稿"
	const addition = "，新输入"
	var draft, preview ui.Field
	draft.SetText(initial)
	snapshots := make(chan string, 2)
	cancelled := make(chan string, 1)
	release := make(chan struct{})
	state := ui.NewEffectKeyed[string, string](func(ctx context.Context, snapshot string) (string, error) {
		snapshots <- snapshot
		if snapshot == initial {
			<-ctx.Done()
			cancelled <- snapshot
			return "过期：" + snapshot, nil
		}
		select {
		case <-release:
			return "预览：" + snapshot, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	var delivered []string
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Children(func() {
			ui.TextField(c, &draft, "草稿", "请输入")
			ui.Effect(c, state, draft.Text())
			if result, err, ok := state.Take(); ok && err == nil {
				delivered = append(delivered, result)
				preview.SetText(result)
			}
			ui.Text(c, preview.Text())
		})
	}})
	var invalidator app.Invalidator
	driver, err := app.NewDriver(page.Config(app.Config{
		Title: "后台文本快照", Width: 480, Height: 240, Invalidator: &invalidator,
	}), benchCanvas())
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	defer page.Close()
	receive := func(values <-chan string) string {
		t.Helper()
		select {
		case value := <-values:
			return value
		case <-time.After(2 * time.Second):
			t.Fatal("background operation did not reach the expected step")
			return ""
		}
	}
	if snapshot := receive(snapshots); snapshot != initial {
		t.Fatalf("worker received %q, want %q", snapshot, initial)
	}
	focused := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "草稿" && !hit.IsLabel {
			if err := driver.Click(center(hit.Bounds)); err != nil {
				t.Fatal(err)
			}
			focused = true
			break
		}
	}
	if !focused {
		t.Fatal("draft field was not available for typing")
	}
	if err := driver.Type(addition); err != nil {
		t.Fatal(err)
	}
	if snapshot := receive(cancelled); snapshot != initial {
		t.Fatalf("typing changed the worker's snapshot to %q", snapshot)
	}
	latest := initial + addition
	if snapshot := receive(snapshots); snapshot != latest {
		t.Fatalf("new worker received %q, want %q", snapshot, latest)
	}
	if preview.Text() != "" {
		t.Fatal("background work directly changed the preview")
	}
	close(release)
	waitEffect(t, func() {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}, func() bool { return preview.Text() != "" })
	want := "预览：" + latest
	if len(delivered) != 1 || delivered[0] != want || draft.Text() != latest {
		t.Fatalf("delivered %q with draft %q, want only %q and unchanged input", delivered, draft.Text(), want)
	}
	for _, label := range driver.Snapshot().Labels {
		if label.Value == want {
			return
		}
	}
	t.Fatal("the latest preview did not reach the rendered page")
}
