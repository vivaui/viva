//go:build darwin && vivahead

// Command typing_latency measures how long a typed character takes to reach
// the screen in a terminal-shaped Viva application: a 110x38 grid of colored
// fixed-pitch rows, a raw-keyboard terminal hit, and a program that echoes what
// it is sent after a configurable delay.
//
// It opens a real window, because presentation timing needs a real display.
// Keys are delivered from inside the process (see darwin.ProbeType), never as
// system events. The native layer prints the statistics on exit when started
// with VIVA_PROFILE_INPUT=1:
//
//	VIVA_PROFILE_INPUT=1 go run -tags vivahead ./benchmarks/typing_latency -echo 0 -keys 120
//
// -echo is the delay before the echo, standing in for the network round trip.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/darwin"
)

const (
	columns    = 110
	rows       = 38
	fontSize   = 13
	scrollRows = rows - 1
)

type terminal struct {
	invalidator app.Invalidator
	echo        time.Duration

	mu      sync.Mutex
	pending []string // echoes that have arrived but are not applied yet
	line    []byte   // what the program has echoed on the last row
	rowRuns [][]gfx.TextRun
	rowText []string
}

func newTerminal(echo time.Duration) *terminal {
	t := &terminal{echo: echo}
	palette := []gfx.Color{
		{Red: 0.62, Green: 0.81, Blue: 0.42, Alpha: 1}, {Red: 0.45, Green: 0.62, Blue: 0.95, Alpha: 1},
		{Red: 0.85, Green: 0.84, Blue: 0.82, Alpha: 1}, {Red: 0.94, Green: 0.47, Blue: 0.47, Alpha: 1},
	}
	for row := 0; row < scrollRows; row++ {
		var text strings.Builder
		var runs []gfx.TextRun
		for column := 0; column < columns-10; {
			word := fmt.Sprintf("word%02d ", (row+column)%97)
			text.WriteString(word)
			runs = append(runs, gfx.TextRun{
				Column: int32(column), Bytes: int32(len(word)), Cells: int32(len(word)),
				Color: palette[(row+column/7)%len(palette)],
			})
			column += len(word)
		}
		t.rowText = append(t.rowText, text.String())
		t.rowRuns = append(t.rowRuns, runs)
	}
	return t
}

func (t *terminal) config() app.Config {
	return app.Config{
		Title: "typing latency", Width: 1000, Height: 700,
		ClearColor:  app.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1},
		Invalidator: &t.invalidator,
		OnWake:      t.apply,
		Paint:       t.paint,
		TextInput: func(context app.TextInputContext) *app.TextInput {
			if !context.Terminal {
				return nil
			}
			t.mu.Lock()
			cursor := len(t.line)
			t.mu.Unlock()
			return &app.TextInput{
				Bounds:   gfx.Rect{X: 8 + float32(cursor+2)*7.8, Y: 8 + float32(scrollRows)*16, Width: 7.8, Height: 16},
				FontSize: fontSize,
				OnEvent: func(event app.TextEvent) (app.TextState, error) {
					if event.Kind == app.TextInsert {
						// The program hears the key now and answers after the delay.
						text := event.Text
						time.AfterFunc(t.echo, func() {
							t.mu.Lock()
							t.pending = append(t.pending, text)
							t.mu.Unlock()
							t.invalidator.Invalidate()
						})
					}
					return app.TextState{}, nil
				},
			}
		},
	}
}

// apply moves arrived echoes into the screen state, on the window thread.
func (t *terminal) apply() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, text := range t.pending {
		t.line = append(t.line, text...)
	}
	t.pending = t.pending[:0]
}

func (t *terminal) paint(canvas *gfx.Canvas, size gfx.Size) {
	cellWidth, cellHeight := canvas.CellSize(fontSize)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1})
	style := gfx.TextStyle{Size: fontSize, Color: gfx.Color{Red: 0.85, Green: 0.84, Blue: 0.82, Alpha: 1}}
	width := float32(columns) * cellWidth
	for row := range t.rowText {
		canvas.LabelRuns(t.rowText[row], gfx.Rect{X: 8, Y: 8 + float32(row)*cellHeight, Width: width, Height: cellHeight}, style, cellWidth, t.rowRuns[row])
	}
	t.mu.Lock()
	prompt := "$ " + string(t.line)
	t.mu.Unlock()
	canvas.LabelRuns(prompt, gfx.Rect{X: 8, Y: 8 + float32(scrollRows)*cellHeight, Width: width, Height: cellHeight}, style, cellWidth,
		[]gfx.TextRun{{Column: 0, Bytes: int32(len(prompt)), Cells: int32(len(prompt)), Color: style.Color}})
	canvas.Rounded(gfx.Rect{X: 8 + float32(len(prompt))*cellWidth, Y: 8 + float32(scrollRows)*cellHeight, Width: cellWidth, Height: cellHeight}, 0,
		gfx.Color{Red: 0.85, Green: 0.84, Blue: 0.82, Alpha: 1})
	canvas.HitWithOptions(1, gfx.Rect{Width: size.Width, Height: size.Height}, "Terminal", gfx.HitOptions{Role: gfx.RoleTerminal, Autofocus: true})
}

func main() {
	echo := flag.Duration("echo", 0, "delay before the program echoes a key (the network round trip)")
	keys := flag.Int("keys", 120, "key presses to type")
	gap := flag.Duration("gap", 70*time.Millisecond, "time between key presses")
	flag.Parse()
	if os.Getenv("VIVA_PROFILE_INPUT") != "1" {
		log.Fatal("set VIVA_PROFILE_INPUT=1: the statistics come from the native layer")
	}
	t := newTerminal(*echo)
	// A probe must never outlive its run: a window left open on someone's
	// screen is worse than a missing number.
	budget := 1500*time.Millisecond + time.Duration(*keys)*(*gap) + 10*time.Second
	time.AfterFunc(budget, func() {
		fmt.Fprintln(os.Stderr, "typing_latency: timed out; the window did not close")
		os.Exit(2)
	})
	started := time.Now()
	defer func() { fmt.Fprintf(os.Stderr, "typing_latency: ran %.1fs\n", time.Since(started).Seconds()) }()
	go func() {
		time.Sleep(1000 * time.Millisecond) // let the window appear
		darwin.ProbeFront()
		time.Sleep(500 * time.Millisecond)
		for key := 0; key < *keys; key++ {
			darwin.ProbeType(string(rune('a' + key%26)))
			time.Sleep(*gap)
		}
		time.Sleep(500 * time.Millisecond)
		darwin.ProbeClose()
	}()
	if err := app.RunCanvas(t.config(), nil); err != nil {
		log.Fatal(err)
	}
}
