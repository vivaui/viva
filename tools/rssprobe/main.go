// Command rssprobe measures what a Viva application keeps resident: it runs a
// child binary, samples the process's resident set size on a cadence, and
// reads the Go heap's live size out of the child's own GC trace. The two
// numbers together separate what the program retains (heap live) from what
// the runtime and the platform hold on to (the rest of RSS), which is the
// split a resident-memory investigation needs and neither number gives alone.
//
// It links nothing into the application: the child is started with
// GODEBUG=gctrace=1 and observed from outside, so the same procedure works on
// a debug build, a release build and another repository's application.
//
// Usage:
//
//	go run ./tools/rssprobe [flags] -- command [args...]
//
// Flags:
//
//	-settle    how long to wait before the first sample (default 2s)
//	-duration  how long to sample (default 10s)
//	-interval  time between samples (default 1s)
//	-out       also write samples as CSV (seconds,rss_kb,heap_live_mb)
//	-label     a scenario name echoed in the summary
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "rssprobe:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("rssprobe", flag.ContinueOnError)
	settle := flags.Duration("settle", 2*time.Second, "how long to wait before the first sample")
	duration := flags.Duration("duration", 10*time.Second, "how long to sample")
	interval := flags.Duration("interval", time.Second, "time between samples")
	out := flags.String("out", "", "write samples as CSV to this path")
	label := flags.String("label", "", "a scenario name echoed in the summary")
	if err := flags.Parse(args); err != nil {
		return err
	}
	rest := flags.Args()
	if len(rest) == 0 {
		return errors.New("no command: rssprobe [flags] -- command [args...]")
	}
	if *interval <= 0 || *duration <= 0 {
		return errors.New("interval and duration must be positive")
	}

	// The GC trace goes to the child's stderr; keep the last heap-live figure.
	trace, traceErr := io.Pipe()
	command := exec.Command(rest[0], rest[1:]...)
	command.Stdout = nil
	command.Stderr = io.MultiWriter(os.Stderr, traceErr)
	command.Env = append(os.Environ(), "GODEBUG=gctrace=1")
	if err := command.Start(); err != nil {
		return err
	}
	heap := newHeapTrace(trace)
	go heap.read()

	samples := sample(command.Process.Pid, heap, *settle, *duration, *interval)
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		fmt.Fprintln(os.Stderr, "rssprobe: killing the child:", err)
	}
	command.Wait()
	traceErr.Close()

	if *out != "" {
		if err := writeCSV(*out, samples, heap); err != nil {
			return err
		}
	}
	summarize(*label, samples, heap)
	return nil
}

type rssSample struct {
	at    time.Duration
	rssKB int64
	heap  float64 // heap live in MB, or the last known value; 0 before the first GC
}

func sample(pid int, heap *heapTrace, settle, duration, interval time.Duration) []rssSample {
	var samples []rssSample
	start := time.Now()
	for {
		until := start.Add(settle + duration)
		if left := time.Until(until); left > 0 {
			time.Sleep(min(left, interval))
		}
		at := time.Since(start)
		if at < settle {
			continue
		}
		if at >= settle+duration {
			return samples
		}
		samples = append(samples, rssSample{at: at, rssKB: residentKB(pid), heap: heap.live()})
	}
}

// residentKB reports the process's resident set size in kilobytes, or -1 when
// the platform query fails or the process is gone.
func residentKB(pid int) int64 {
	switch runtime.GOOS {
	case "darwin", "linux":
		out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			return -1
		}
		kb, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return -1
		}
		return kb
	case "windows":
		// tasklist quotes fields and formats memory as "12,345 K".
		out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
		if err != nil {
			return -1
		}
		fields := strings.Split(strings.TrimSpace(string(out)), "\",\"")
		if len(fields) < 5 || !strings.HasSuffix(fields[4], " K") {
			return -1
		}
		kb, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSuffix(fields[4], " K"), ",", ""), 10, 64)
		if err != nil {
			return -1
		}
		return kb
	}
	return -1
}

// summarize prints the numbers a report keeps: resident memory at its steadiest
// and at its worst, and the heap the program actually lives on.
func summarize(label string, samples []rssSample, heap *heapTrace) {
	if label != "" {
		fmt.Printf("label %s\n", label)
	}
	if len(samples) == 0 {
		fmt.Println("no samples")
		return
	}
	rss := make([]int64, 0, len(samples))
	for _, at := range samples {
		if at.rssKB >= 0 {
			rss = append(rss, at.rssKB)
		}
	}
	sort.Slice(rss, func(i, j int) bool { return rss[i] < rss[j] })
	fmt.Printf("samples %d over %s\n", len(samples), samples[len(samples)-1].at.Round(time.Second))
	if len(rss) > 0 {
		fmt.Printf("rss min %d KB, median %d KB, max %d KB, final %d KB\n",
			rss[0], rss[len(rss)/2], rss[len(rss)-1], rss[len(rss)-1])
	} else {
		fmt.Println("rss unavailable")
	}
	if mb := heap.live(); mb > 0 {
		fmt.Printf("heap live %.1f MB\n", mb)
	} else {
		fmt.Println("heap live unknown: the child has not run a GC")
		fmt.Println("  (a settled idle window may never allocate one; the burst and")
		fmt.Println("   close scenarios do, and the figure appears from their first GC)")
	}
}

func writeCSV(path string, samples []rssSample, heap *heapTrace) error {
	var b strings.Builder
	b.WriteString("seconds,rss_kb,heap_live_mb\n")
	for _, at := range samples {
		fmt.Fprintf(&b, "%.1f,%d,%.1f\n", at.at.Seconds(), at.rssKB, at.heap)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// heapTrace keeps the last heap-live figure out of a stream of GC trace lines.
type heapTrace struct {
	source io.Reader
	liveMB float64
}

func newHeapTrace(reader io.Reader) *heapTrace {
	return &heapTrace{source: reader}
}

var heapTriple = regexp.MustCompile(`(\d+)->(\d+)->(\d+) MB`)

func (trace *heapTrace) read() {
	buf := make([]byte, 4096)
	var pending []byte
	for {
		n, err := trace.source.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			for {
				lineEnd := bytes.IndexByte(pending, '\n')
				if lineEnd < 0 {
					break
				}
				trace.line(string(pending[:lineEnd]))
				pending = pending[lineEnd+1:]
			}
			// A program that writes no newlines at all cannot be a GC trace;
			// do not grow without bound waiting for one.
			if len(pending) > 64<<10 {
				pending = pending[len(pending)-4096:]
			}
		}
		if err != nil {
			return
		}
	}
}

// line reads one GC trace line: `gc 8 @2.163s 2%: ... 17->20->12 MB, 20 MB
// goal, ...`, whose third figure is the heap that survived the collection.
func (trace *heapTrace) line(text string) {
	if !strings.HasPrefix(text, "gc ") {
		return
	}
	match := heapTriple.FindStringSubmatch(text)
	if match == nil {
		return
	}
	if mb, err := strconv.ParseFloat(match[3], 64); err == nil {
		trace.liveMB = mb
	}
}

// live reports the most recent heap-live figure. Between the child's GCs the
// last figure stands: the sampler copies it into each sample it takes.
func (trace *heapTrace) live() float64 {
	return trace.liveMB
}
