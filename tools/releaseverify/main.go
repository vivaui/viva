// Command releaseverify checks a published Viva release against the notes that
// were committed for it, the tag that was pushed and the assets the workflow
// built, and lists the "current release" references that still name an older
// version. It only reads; the release itself is never changed.
//
//	go run ./tools/releaseverify -tag v0.2.2
//
// It needs the gh and git commands, and the tag fetched into the checkout
// (git fetch --tags origin).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

const defaultRepo = "vivaui/viva"

// references are the files that name the current release (docs/releases.md).
var references = []string{
	"README.md",
	"README.zh-CN.md",
	"website/content/guides/en/getting-started.md",
	"website/content/guides/zh/getting-started.md",
}

type asset struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type release struct {
	Name         string  `json:"name"`
	TagName      string  `json:"tagName"`
	IsDraft      bool    `json:"isDraft"`
	IsPrerelease bool    `json:"isPrerelease"`
	Body         string  `json:"body"`
	Assets       []asset `json:"assets"`
}

// deps is what the checks read from outside, so the tests can supply it.
type deps struct {
	gh  func(args ...string) (string, error)
	git func(args ...string) (string, error)
}

// finding is one line of the report. A failed finding makes the command exit
// with a status of 1; a pending one is work left for the person releasing.
type finding struct {
	ok      bool
	pending bool
	text    string
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "releaseverify:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("releaseverify", flag.ContinueOnError)
	tag := flags.String("tag", "", "release tag, for example v0.2.2")
	repo := flags.String("repo", defaultRepo, "GitHub repository")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !versionTag.MatchString(*tag) || flags.NArg() != 0 {
		return fmt.Errorf("usage: releaseverify -tag vX.Y.Z [-repo OWNER/NAME]")
	}
	findings, err := verify(*tag, *repo, deps{gh: command("gh"), git: command("git")})
	if err != nil {
		return err
	}
	failed := 0
	for _, f := range findings {
		switch {
		case f.ok:
			fmt.Fprintln(out, "ok     ", f.text)
		case f.pending:
			fmt.Fprintln(out, "todo   ", f.text)
		default:
			failed++
			fmt.Fprintln(out, "FAIL   ", f.text)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

var versionTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

func command(name string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return string(out), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return string(out), nil
	}
}

func verify(tag, repo string, d deps) ([]finding, error) {
	var findings []finding
	add := func(ok bool, format string, args ...any) {
		findings = append(findings, finding{ok: ok, text: fmt.Sprintf(format, args...)})
	}
	raw, err := d.gh("release", "view", tag, "--repo", repo, "--json", "name,tagName,isDraft,isPrerelease,body,assets")
	if err != nil {
		return nil, err
	}
	rel, err := parseRelease(raw)
	if err != nil {
		return nil, err
	}

	prerelease := strings.Contains(tag, "-")
	add(rel.TagName == tag, "the release is for %s (got %s)", tag, rel.TagName)
	add(rel.Name == "Viva "+tag, "the title is %q (got %q)", "Viva "+tag, rel.Name)
	add(!rel.IsDraft, "the release is published, not a draft")
	add(rel.IsPrerelease == prerelease, "prerelease is %v, as the tag says (got %v)", prerelease, rel.IsPrerelease)

	missing, extra := compareAssets(rel.Assets, expectedAssets(tag))
	add(len(missing) == 0 && len(extra) == 0, "the assets are exactly what the workflow builds (missing: %s; unexpected: %s)", list(missing), list(extra))

	sums, err := d.gh("release", "download", tag, "--repo", repo, "--pattern", "SHA256SUMS", "--output", "-")
	if err != nil {
		add(false, "SHA256SUMS could not be downloaded: %v", err)
	} else {
		problems := digestMismatches(rel.Assets, sums)
		for _, line := range problems {
			add(false, "%s", line)
		}
		if len(problems) == 0 {
			add(true, "every archive's GitHub digest equals its line in SHA256SUMS")
		}
	}

	commit, annotated, err := tagCommit(d, tag)
	if err != nil {
		add(false, "the tag: %v", err)
	} else {
		add(annotated, "%s is an annotated tag, on commit %s", tag, commit)
		notes, err := d.git("show", commit+":docs/releases/"+strings.TrimPrefix(tag, "v")+".md")
		switch {
		case err != nil:
			add(false, "no docs/releases/%s.md at the tag commit: %v", strings.TrimPrefix(tag, "v"), err)
		case normalize(notes) != normalize(rel.Body):
			add(false, "the release body differs from docs/releases/%s.md at the tag commit", strings.TrimPrefix(tag, "v"))
		default:
			add(true, "the release body is docs/releases/%s.md at the tag commit", strings.TrimPrefix(tag, "v"))
		}
	}

	if !prerelease {
		for _, file := range references {
			text, err := d.git("show", "origin/main:"+file)
			if err != nil {
				add(false, "%s could not be read at origin/main: %v", file, err)
				continue
			}
			if strings.Contains(text, "`"+tag+"`") {
				add(true, "%s names %s", file, tag)
			} else {
				findings = append(findings, finding{pending: true, text: file + " does not name " + tag + " yet"})
			}
		}
	}
	return findings, nil
}

func parseRelease(raw string) (release, error) {
	var rel release
	if err := json.Unmarshal([]byte(raw), &rel); err != nil {
		return rel, fmt.Errorf("gh release view: %w", err)
	}
	return rel, nil
}

// expectedAssets is what release.yml publishes for a tag: six archives of the
// viva command and SHA256SUMS, plus the Homebrew formula and the Scoop manifest
// unless the tag is a prerelease.
func expectedAssets(tag string) []string {
	var names []string
	for _, os := range []string{"darwin", "linux", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			extension := "tar.gz"
			if os == "windows" {
				extension = "zip"
			}
			names = append(names, fmt.Sprintf("viva_%s_%s_%s.%s", tag, os, arch, extension))
		}
	}
	names = append(names, "SHA256SUMS")
	if !strings.Contains(tag, "-") {
		names = append(names, "viva.rb", "viva.json")
	}
	sort.Strings(names)
	return names
}

func compareAssets(got []asset, want []string) (missing, extra []string) {
	have := map[string]bool{}
	for _, a := range got {
		have[a.Name] = true
	}
	wanted := map[string]bool{}
	for _, name := range want {
		wanted[name] = true
		if !have[name] {
			missing = append(missing, name)
		}
	}
	for _, a := range got {
		if !wanted[a.Name] {
			extra = append(extra, a.Name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

// digestMismatches compares the digest GitHub computed for each archive with
// the line SHA256SUMS holds for it.
func digestMismatches(assets []asset, sums string) []string {
	listed := map[string]string{}
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			listed[strings.TrimPrefix(fields[1], "*")] = fields[0]
		}
	}
	var problems []string
	for _, a := range assets {
		if !strings.HasPrefix(a.Name, "viva_v") {
			continue
		}
		sum, ok := listed[a.Name]
		switch {
		case !ok:
			problems = append(problems, a.Name+" has no line in SHA256SUMS")
		case a.Digest != "" && a.Digest != "sha256:"+sum:
			problems = append(problems, fmt.Sprintf("%s: GitHub says %s, SHA256SUMS says sha256:%s", a.Name, a.Digest, sum))
		}
	}
	sort.Strings(problems)
	return problems
}

// tagCommit is the commit a tag points at, and whether the tag is an annotated
// object of its own (ls-remote lists the peeled commit only for those).
func tagCommit(d deps, tag string) (commit string, annotated bool, err error) {
	out, err := d.git("ls-remote", "--tags", "origin", "refs/tags/"+tag, "refs/tags/"+tag+"^{}")
	if err != nil {
		return "", false, err
	}
	var object, peeled string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[1] == "refs/tags/"+tag {
			object = fields[0]
		}
		if fields[1] == "refs/tags/"+tag+"^{}" {
			peeled = fields[0]
		}
	}
	switch {
	case object == "":
		return "", false, fmt.Errorf("origin has no tag %s", tag)
	case peeled == "":
		return object, false, nil
	}
	return peeled, true, nil
}

// normalize makes two copies of the same notes comparable: GitHub stores a body
// with its line endings and trailing blank lines as the web form or the API
// left them.
func normalize(text string) string {
	return strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), " \t\n")
}

func list(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}
