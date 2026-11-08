package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//go:embed check_test.go.txt
var checkTest string

// Attempt is one model reply and what the compiler said about it.
type Attempt struct {
	N      int
	Finish string
	Vet    string // empty when it passed; else the start of go vet's or viva vet's output
}

// Check is the verdict on one expectation.
type Check struct {
	Name   string
	OK     bool
	Detail string `json:",omitempty"`
}

// Run is one task, one model, one sample.
type Run struct {
	Task, Model  string
	Index        int
	Needs        string `json:",omitempty"`
	Attempts     []Attempt
	Compiled     bool
	FirstTry     bool
	RepairRounds int // go vet and viva vet failures survived before it passed both
	Checks       []Check
	Pass         bool // compiled and every check held
	Prompt       int
	Completion   int
	Seconds      float64
	Error        string `json:",omitempty"`
}

type env struct {
	vivaRoot, vivaBin, out string
	repairs                int
	png                    bool
	scrub                  func(string) string
}

const systemIntro = "You write Go desktop UIs with the Viva framework. Here is the project guide (AGENTS.md):\n\n"

func systemPrompt(guide, template string) string {
	return systemIntro + guide +
		"\n\nHere is the project's current main.go, a working example:\n\n```go\n" + template + "```\n\n" +
		"Reply with ONE complete replacement main.go in a single ```go block and nothing else. Keep `package main`, a `type model struct`, " +
		"`func newModel() *model` (initial data), `func newConfig(m *model) app.Config` and `main()` as in the example (the window size from the task goes in app.Config Width/Height). " +
		"Use only APIs the guide or the example shows or that you are sure exist in packages github.com/vivaui/viva/app, github.com/vivaui/viva/ui, github.com/vivaui/viva/flex, github.com/vivaui/viva/components. No third-party packages."
}

func sh(dir string, timeout time.Duration, extraEnv []string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (e *env) runJob(task Task, m Model, key string, index int) Run {
	start := time.Now()
	run := e.attempt(task, m, key, index)
	run.Seconds = time.Since(start).Seconds()
	return run
}

func (e *env) attempt(task Task, m Model, key string, index int) Run {
	run := Run{Task: task.Name, Model: m.Name, Index: index, Needs: task.Needs}
	fail := func(format string, args ...any) Run {
		run.Error = e.scrub(fmt.Sprintf(format, args...))
		return run
	}

	base := filepath.Join(e.out, "raw", m.Name, task.Name, fmt.Sprintf("run%d", index))
	project := filepath.Join(base, "proj")
	os.RemoveAll(base)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fail("%v", err)
	}
	if out, err := sh(e.vivaRoot, 2*time.Minute, nil, e.vivaBin, "init", "-viva", e.vivaRoot, "-tidy=false", project); err != nil {
		return fail("viva init: %v\n%s", err, out)
	}
	sum, err := os.ReadFile(filepath.Join(e.vivaRoot, "go.sum"))
	if err == nil {
		err = os.WriteFile(filepath.Join(project, "go.sum"), sum, 0o644)
	}
	if err != nil {
		return fail("%v", err)
	}
	// The generated test asserts the template's counter; the model replaces the page.
	os.Remove(filepath.Join(project, "main_test.go"))
	taskJSON, _ := json.Marshal(task.Ops)
	resultPath := filepath.Join(base, "result.json")
	if err := os.WriteFile(filepath.Join(base, "task.json"), taskJSON, 0o644); err != nil {
		return fail("%v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "aieval_check_test.go"), []byte(checkTest), 0o644); err != nil {
		return fail("%v", err)
	}
	guide, _ := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	template, _ := os.ReadFile(filepath.Join(project, "main.go"))

	messages := []message{
		{"system", systemPrompt(string(guide), string(template))},
		{"user", "Task: " + task.Request},
	}
	for attempt := 1; attempt <= e.repairs+1; attempt++ {
		reply, finish, u, err := chat(m, key, messages)
		run.Prompt += u.Prompt
		run.Completion += u.Completion
		if err != nil {
			return fail("chat: %v", err)
		}
		os.WriteFile(filepath.Join(base, fmt.Sprintf("reply%d.txt", attempt)), []byte(e.scrub(reply)), 0o644)
		a := Attempt{N: attempt, Finish: finish}
		code, ok := extractGo(reply)
		var vet string
		if !ok {
			vet = "the reply has no ```go block"
		} else {
			if err := os.WriteFile(filepath.Join(project, "main.go"), []byte(code), 0o644); err != nil {
				return fail("%v", err)
			}
			out, err := sh(project, 5*time.Minute, nil, "go", "vet", "./...")
			if err == nil {
				// What compiles may still bind a copy or leave a list unkeyed:
				// viva vet's findings go back to the model as the compiler's do,
				// as the guide tells a user to run it.
				out, err = sh(project, 5*time.Minute, nil, e.vivaBin, "vet", "./...")
			}
			if err != nil {
				vet = strings.TrimSpace(out)
				if vet == "" {
					vet = err.Error()
				}
			}
		}
		if vet == "" {
			run.Attempts = append(run.Attempts, a)
			run.Compiled, run.FirstTry, run.RepairRounds = true, attempt == 1, attempt-1
			break
		}
		a.Vet = e.scrub(truncate(vet, 1500))
		run.Attempts = append(run.Attempts, a)
		messages = append(messages, message{"assistant", reply},
			message{"user", "go vet and viva vet reported:\n" + truncate(vet, 3000) + "\nFix it and reply with the complete main.go."})
	}
	if !run.Compiled {
		return run
	}

	run.Checks = e.check(project, resultPath, base, task)
	run.Pass = true
	for _, c := range run.Checks {
		run.Pass = run.Pass && c.OK
	}
	return run
}

func (e *env) check(project, resultPath, base string, task Task) []Check {
	var checks []Check
	source, _ := os.ReadFile(filepath.Join(project, "main.go"))
	for _, s := range task.Source {
		c := Check{Name: "source " + s, OK: strings.Contains(string(source), s)}
		if !c.OK {
			c.Detail = "main.go does not contain it"
		}
		checks = append(checks, c)
	}
	args := []string{"test", "-count=1", "-timeout", "120s", "-run", "TestAIEvalCheck"}
	environment := []string{"AIEVAL_TASK=" + filepath.Join(base, "task.json"), "AIEVAL_RESULT=" + resultPath}
	if e.png {
		args = append(args, "-tags", "vivahead")
		environment = append(environment, "AIEVAL_PNG="+filepath.Join(base, "shot.png"))
	}
	out, err := sh(project, 3*time.Minute, environment, "go", append(args, "./...")...)
	data, readErr := os.ReadFile(resultPath)
	var results []Check
	if readErr == nil {
		json.Unmarshal(data, &results)
	}
	if err != nil && len(results) == 0 {
		return append(checks, Check{Name: "run", Detail: e.scrub(truncate(out, 800))})
	}
	if readErr != nil && len(task.Ops) > 0 {
		return append(checks, Check{Name: "run", Detail: "no result: " + e.scrub(truncate(out, 400))})
	}
	return append(checks, results...)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
