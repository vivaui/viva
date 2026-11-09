package devreload

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/reloadwire"
)

type Options struct {
	Directory     string
	Package       string
	Arguments     []string
	Debounce      time.Duration
	PollInterval  time.Duration
	BuildTimeout  time.Duration
	WorkerTimeout time.Duration
	Output        io.Writer
	ErrorOutput   io.Writer
}

type builder struct {
	root      string
	packageID string
	directory string
	sequence  atomic.Uint64
	timeout   time.Duration
	output    io.Writer
	errorOut  io.Writer
}

func Run(ctx context.Context, options Options) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if options.Package == "" {
		return fmt.Errorf("dev requires a Go main package")
	}
	root, err := moduleRoot(options.Directory)
	if err != nil {
		return fmt.Errorf("find module root from %s: %w", options.Directory, err)
	}
	temporary, err := os.MkdirTemp("", "viva-reload-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	build := &builder{
		root: root, packageID: options.Package, directory: temporary, timeout: options.BuildTimeout,
		output: options.Output, errorOut: options.ErrorOutput,
	}
	binary, err := build.worker(ctx)
	if err != nil {
		return err
	}
	worker, err := startWorker(ctx, binary, options.Arguments, options.Output, options.ErrorOutput, options.WorkerTimeout)
	if err != nil {
		_ = os.Remove(binary)
		return err
	}
	request := reloadwire.PaintRequest{Width: float32(worker.config.Width), Height: float32(worker.config.Height)}
	paintCtx, paintCancel := context.WithTimeout(ctx, options.WorkerTimeout)
	snapshot, err := worker.client.Paint(paintCtx, request)
	paintCancel()
	if err != nil {
		worker.stop()
		return fmt.Errorf("initial worker validation: %w", err)
	}
	supervisor := newSupervisor(worker, request, snapshot, options.ErrorOutput)
	defer supervisor.stop()

	watch := newWatcher(root, options.PollInterval)
	watchDone := make(chan error, 1)
	go func() { watchDone <- watch.run(ctx) }()
	reloadDone := make(chan struct{})
	go func() {
		defer close(reloadDone)
		reloadLoop(ctx, watch.changes, options.Debounce, build, options, supervisor)
	}()

	canvas := &gfx.Canvas{}
	reloadwire.ApplyCanvas(canvas, snapshot)
	fmt.Fprintf(options.Output, "viva dev: running %s\n", options.Package)
	runErr := app.RunCanvas(supervisor.hostConfig(), canvas)
	cancel()
	<-reloadDone
	if watchErr := <-watchDone; watchErr != nil && runErr == nil {
		runErr = watchErr
	}
	return runErr
}

func reloadLoop(ctx context.Context, changes <-chan struct{}, debounce time.Duration, build *builder, options Options, supervisor *supervisor) {
	var timer *time.Timer
	var timerChannel <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-changes:
			if timer == nil {
				timer = time.NewTimer(debounce)
			} else if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
				timer.Reset(debounce)
			} else {
				timer.Reset(debounce)
			}
			timerChannel = timer.C
		case <-timerChannel:
			timerChannel = nil
			binary, err := build.worker(ctx)
			if err != nil {
				fmt.Fprintf(options.ErrorOutput, "viva dev: build failed; previous worker remains active\n%v\n", err)
				continue
			}
			candidate, err := startWorker(ctx, binary, options.Arguments, options.Output, options.ErrorOutput, options.WorkerTimeout)
			if err != nil {
				_ = os.Remove(binary)
				fmt.Fprintf(options.ErrorOutput, "viva dev: candidate failed; previous worker remains active: %v\n", err)
				continue
			}
			if err := supervisor.swap(candidate); err != nil {
				candidate.stop()
				fmt.Fprintf(options.ErrorOutput, "viva dev: candidate rejected; previous worker remains active: %v\n", err)
				continue
			}
			fmt.Fprintln(options.Output, "viva dev: reload complete")
		}
	}
}

func (build *builder) worker(ctx context.Context) (string, error) {
	sequence := build.sequence.Add(1)
	path := filepath.Join(build.directory, fmt.Sprintf("worker-%06d", sequence))
	if filepath.Separator == '\\' {
		path += ".exe"
	}
	buildCtx, cancel := context.WithTimeout(ctx, build.timeout)
	defer cancel()
	command := exec.CommandContext(buildCtx, "go", "build", "-trimpath", "-tags", "vivadev", "-o", path, build.packageID)
	command.Dir = build.root
	var diagnostics bytes.Buffer
	command.Stdout = &diagnostics
	command.Stderr = &diagnostics
	started := time.Now()
	if err := command.Run(); err != nil {
		_ = os.Remove(path)
		if diagnostics.Len() == 0 {
			return "", err
		}
		return "", fmt.Errorf("%w\n%s", err, diagnostics.String())
	}
	fmt.Fprintf(build.output, "viva dev: built in %s\n", time.Since(started).Round(time.Millisecond))
	return path, nil
}
