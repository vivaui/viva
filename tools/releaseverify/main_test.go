package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func goodRelease(tag string) release {
	rel := release{Name: "Viva " + tag, TagName: tag, Body: "notes\n"}
	for _, name := range expectedAssets(tag) {
		rel.Assets = append(rel.Assets, asset{Name: name, Digest: "sha256:" + strings.Repeat("a", 64)})
	}
	return rel
}

func sumsFor(rel release) string {
	var lines []string
	for _, a := range rel.Assets {
		if strings.HasPrefix(a.Name, "viva_v") {
			lines = append(lines, strings.Repeat("a", 64)+"  "+a.Name)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func fakeDeps(t *testing.T, tag string, rel release, mutate func(*fakes)) deps {
	t.Helper()
	f := &fakes{rel: rel, sums: sumsFor(rel), tagLines: "1111 refs/tags/" + tag + "\n2222 refs/tags/" + tag + "^{}\n", notes: rel.Body, current: "`" + tag + "`"}
	if mutate != nil {
		mutate(f)
	}
	return f.deps(tag)
}

type fakes struct {
	rel      release
	sums     string
	tagLines string
	notes    string
	current  string
	noNotes  bool
}

func (f *fakes) deps(tag string) deps {
	return deps{
		gh: func(args ...string) (string, error) {
			if args[0] == "release" && args[1] == "download" {
				return f.sums, nil
			}
			body := strings.ReplaceAll(f.rel.Body, "\n", `\n`)
			out := `{"name":"` + f.rel.Name + `","tagName":"` + f.rel.TagName + `","isDraft":` + map[bool]string{true: "true", false: "false"}[f.rel.IsDraft] + `,"isPrerelease":` + map[bool]string{true: "true", false: "false"}[f.rel.IsPrerelease] + `,"body":"` + body + `","assets":[`
			for i, a := range f.rel.Assets {
				if i > 0 {
					out += ","
				}
				out += `{"name":"` + a.Name + `","digest":"` + a.Digest + `"}`
			}
			return out + "]}", nil
		},
		git: func(args ...string) (string, error) {
			switch args[0] {
			case "ls-remote":
				return f.tagLines, nil
			case "show":
				if strings.Contains(args[1], "docs/releases/") {
					if f.noNotes {
						return "", errors.New("missing")
					}
					return f.notes, nil
				}
				return f.current, nil
			}
			return "", errors.New("unexpected git call")
		},
	}
}

func failures(findings []finding) []string {
	var out []string
	for _, f := range findings {
		if !f.ok && !f.pending {
			out = append(out, f.text)
		}
	}
	return out
}

func TestACompleteReleaseVerifies(t *testing.T) {
	rel := goodRelease("v0.2.2")
	findings, err := verify("v0.2.2", defaultRepo, fakeDeps(t, "v0.2.2", rel, nil))
	if err != nil || len(failures(findings)) != 0 {
		t.Fatalf("err=%v failures=%v", err, failures(findings))
	}
}

func TestEachDefectOfARelease(t *testing.T) {
	for name, tc := range map[string]struct {
		change func(*release)
		fakes  func(*fakes)
		want   string
	}{
		"wrong title":   {change: func(r *release) { r.Name = "v0.2.2" }, want: "the title"},
		"still a draft": {change: func(r *release) { r.IsDraft = true }, want: "not a draft"},
		"extra asset":   {change: func(r *release) { r.Assets = append(r.Assets, asset{Name: "notes.txt"}) }, want: "unexpected: notes.txt"},
		"missing asset": {change: func(r *release) { r.Assets = r.Assets[1:] }, want: "missing:"},
		"digest differs": {fakes: func(f *fakes) {
			for i := range f.rel.Assets {
				if strings.HasPrefix(f.rel.Assets[i].Name, "viva_v") {
					f.rel.Assets[i].Digest = "sha256:" + strings.Repeat("b", 64)
					return
				}
			}
		}, want: "GitHub says"},
		"lightweight tag": {fakes: func(f *fakes) { f.tagLines = "1111 refs/tags/v0.2.2\n" }, want: "annotated"},
		"notes differ":    {fakes: func(f *fakes) { f.notes = "other\n" }, want: "differs"},
		"no notes file":   {fakes: func(f *fakes) { f.noNotes = true }, want: "no docs/releases/0.2.2.md"},
	} {
		t.Run(name, func(t *testing.T) {
			rel := goodRelease("v0.2.2")
			if tc.change != nil {
				tc.change(&rel)
			}
			findings, err := verify("v0.2.2", defaultRepo, fakeDeps(t, "v0.2.2", rel, tc.fakes))
			if err != nil {
				t.Fatal(err)
			}
			got := failures(findings)
			if !slices.ContainsFunc(got, func(s string) bool { return strings.Contains(s, tc.want) }) {
				t.Fatalf("failures %v do not mention %q", got, tc.want)
			}
		})
	}
}

// A prerelease carries no Homebrew or Scoop manifest and is not announced
// through the references to the current release.
func TestAPrereleaseHasNoPackageManifests(t *testing.T) {
	names := expectedAssets("v0.3.0-rc.1")
	if slices.Contains(names, "viva.rb") || slices.Contains(names, "viva.json") || len(names) != 7 {
		t.Fatalf("prerelease assets = %v", names)
	}
	if got := expectedAssets("v0.2.2"); len(got) != 9 || !slices.Contains(got, "viva_v0.2.2_windows_arm64.zip") {
		t.Fatalf("release assets = %v", got)
	}
}

func TestReferencesToTheCurrentReleaseAreReportedNotFailed(t *testing.T) {
	rel := goodRelease("v0.2.2")
	findings, err := verify("v0.2.2", defaultRepo, fakeDeps(t, "v0.2.2", rel, func(f *fakes) { f.current = "`v0.2.1`" }))
	if err != nil || len(failures(findings)) != 0 {
		t.Fatalf("err=%v failures=%v", err, failures(findings))
	}
	pending := 0
	for _, f := range findings {
		if f.pending {
			pending++
		}
	}
	if pending != len(references) {
		t.Fatalf("%d references pending, want %d", pending, len(references))
	}
}

func TestNotesCompareWithoutLineEndingAndTrailingBlankDifferences(t *testing.T) {
	if normalize("a\r\nb\n\n") != normalize("a\nb") {
		t.Fatal("equal notes compare unequal")
	}
	if normalize("a\nb") == normalize("a\nc") {
		t.Fatal("different notes compare equal")
	}
}

func TestUsage(t *testing.T) {
	var out strings.Builder
	for _, args := range [][]string{{}, {"-tag", "0.2.2"}, {"-tag", "v0.2.2", "extra"}} {
		if err := run(args, &out); err == nil {
			t.Errorf("run(%v) accepted bad arguments", args)
		}
	}
}
