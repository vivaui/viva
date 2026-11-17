package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskFileParsesOrderedOps(t *testing.T) {
	task, err := parseTask("x", "---\ntitle: T\nneeds: modal\ntext: Name\nclick: Save\nnot-text: Name\nsource: OpenWindow\n---\nMake a page.\n")
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "T" || task.Needs != "modal" || task.Request != "Make a page." || len(task.Source) != 1 {
		t.Fatalf("%+v", task)
	}
	if len(task.Ops) != 3 || task.Ops[1] != (Op{"click", "Save"}) {
		t.Fatalf("ops out of order: %+v", task.Ops)
	}
	if _, err := parseTask("x", "---\ntitle: T\nbogus: 1\n---\nr"); err == nil {
		t.Fatal("an unknown directive must be an error")
	}
}

func TestExtractGoTakesTheLongestBlockAndAnUnterminatedOne(t *testing.T) {
	if got, _ := extractGo("a\n```go\nx\n```\nb\n```go\nlonger\n```"); got != "longer\n" {
		t.Fatalf("got %q", got)
	}
	if got, ok := extractGo("```go\npackage main\nfunc"); !ok || !strings.HasPrefix(got, "package main") {
		t.Fatalf("a reply cut off by the token limit must still be repairable: %q", got)
	}
}

func TestModelConfigAndFlag(t *testing.T) {
	m, err := parseModelFlag(`fast=https://api.example.com/v1,MY_KEY,{"thinking":{"type":"disabled"}}`)
	if err != nil || m.BaseURL != "https://api.example.com/v1" || m.KeyEnv != "MY_KEY" || !strings.Contains(m.Extra, "disabled") {
		t.Fatalf("%+v %v", m, err)
	}
	if _, err := parseModelFlag("nokey=https://x"); err == nil {
		t.Fatal("a model without a key variable must be refused")
	}
}

// Every task is a request a user would make; the generated project guide is the
// only document aieval sends, so each task must have a section there that tells
// the model where to start. A task whose group is missing from this map is a
// gap in the guide, not just in the test.
var taskRoutes = map[string]string{
	"form": "Forms and fields", "login": "Forms and fields", "wizard": "Forms and fields",
	"settings": "Forms and fields", "steppers": "Forms and fields", "search": "Forms and fields",
	"todo": "Lists, trees and scrolling", "cart": "Lists, trees and scrolling",
	"inbox": "Lists, trees and scrolling", "notifications": "Lists, trees and scrolling",
	"logviewer": "Lists, trees and scrolling", "kanban": "Lists, trees and scrolling",
	"masterdetail": "Lists, trees and scrolling", "calendar": "Lists, trees and scrolling",
	"filetree": "Lists, trees and scrolling", "gallery": "Lists, trees and scrolling",
	"chat":  "Lists, trees and scrolling",
	"table": "Tables",
	"tabs":  "Tabs and select",
	"modal": "Menus, dialogs and popups", "dashboard": "Menus, dialogs and popups",
	"progress": "Animation and time", "pomodoro": "Animation and time", "mediaplayer": "Animation and time",
	"darktheme": "The theme", "colorpicker": "The theme",
	"splitter":   "Keys, drag and the mouse",
	"twowindows": "Several windows",
}

func TestEveryTaskHasAGuideRoute(t *testing.T) {
	guide, err := os.ReadFile("../../cmd/viva/initfiles/AGENTS.md.txt")
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("tasks/*.md")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, file := range files {
		task := strings.TrimSuffix(filepath.Base(file), ".md")
		seen[task] = true
		section, ok := taskRoutes[task]
		if !ok {
			t.Errorf("task %s has no guide route; add it to taskRoutes and a section to the generated guide", task)
			continue
		}
		if !strings.Contains(string(guide), "## "+section) {
			t.Errorf("task %s routes to %q, which the generated guide does not have", task, section)
		}
	}
	for task := range taskRoutes {
		if !seen[task] {
			t.Errorf("taskRoutes names %s, but tools/aieval/tasks has no such task", task)
		}
	}
}
