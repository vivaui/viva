//go:build darwin || windows || linux

package headless

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// Serve runs the line protocol on in and out until "quit" or the end of input.
// Each request is one line; the reply is zero or more data lines followed by
// exactly one final line, "ok ..." or "error ...". Arguments are separated by
// spaces; an argument containing spaces is written in double quotes ("New
// Host"), with \" and \\ as escapes. Coordinates are logical pixels.
//
//	size                       ok 1400x900 scale=2
//	move X Y | down X Y | up X Y | click X Y | dblclick X Y | rightclick X Y
//	scroll X Y DX DY [MODIFIERS]  scroll at X,Y; modifiers are shift+ctrl+alt+cmd
//	drop X Y PATH...           drop files from the system onto the point: ok accepted|refused
//	dragover X Y | dragleave   the hover of such a drag, and its end
//	click "Label"              click the center of the control with that label
//	dblclick "Label"           double-click the center of that control
//	rightclick "Label"         right-click the center of that control (opens its context menu)
//	key SPEC...                tab | shift+tab | enter | esc | backspace | left |
//	                           pageup | f5 | insert | cmd+a | cmd+c | cmd+x | cmd+v | x ...
//	                           ctrl+c and configured alt+x chords go to a focused terminal
//	keydown NAME | keyup NAME  hold and release one key: a named key ("left", "f5"), a
//	                           character, or a modifier (shift ctrl alt cmd, rshift rctrl
//	                           ralt rcmd). Held modifiers apply to every click, scroll and
//	                           key until released. A terminal that asked for key releases
//	                           sees the presses and releases; elsewhere keydown acts as key
//	blur                       the window loses the keyboard: held keys are released
//	type TEXT                  insert the rest of the line into the focused field
//	compose [TEXT]             set the input-method composition of the focused field to the
//	                           rest of the line, as an input method does while a syllable is
//	                           typed; without TEXT the composition ends. type commits text
//	select START END           select a rune range of the focused field
//	resize W H
//	frame                     ok x=.. y=.. w=.. h=.. (the content area on screen)
//	windowmove X Y            move the window's content area to a screen point, as a drag
//	                          of the window would, and report its new frame to OnFrameChange
//	minimize on|off           report the window as minimized or restored, as the window
//	                          manager would, and call OnMinimize
//	maximize on|off           report the window as maximized or restored, as the window
//	                          manager would, and call OnMaximize
//	wait MS                    advance the animation clock
//	settle [MS]                apply background work until quiet for MS (default 100)
//	waitfor "text" [MS]        wait up to MS (default 5000) real ms for text on screen;
//	                           that time also passes for the app's timers
//	shot PATH [now]            write a PNG of the interface at rest: transitions in flight
//	                           (a dialog fading in) are played to their end first. With
//	                           now, write the current frame as it is, so wait MS; shot
//	                           PATH now catches an animation MS into it
//	record DIR MS [FPS]        play MS more of animation and write it to DIR as a PNG
//	                           sequence and manifest.json, sampling FPS frames a second
//	                           (default 60); frames that did not change are written once
//	                           with a longer duration, and ok reports the count
//	find "label"               ok id=7 x=.. y=.. w=.. h=.. label=".."
//	hits [filter]              one line per control, then ok
//	texts [filter]             one line per painted text, then ok
//	layout                     one line per suspicious spot (text cut off, spilling out of its
//	                           control, over other text, off the window; focusable controls that
//	                           cross), then ok N problems; it never fails
//	focus                      ok id=.. label=".." (id=0 when nothing is focused)
//	editor                     ok value=".." selection=a,b (no editor: ok none)
//	appearance [light|dark|unknown]  read or set the system appearance the run reports
//	clip [TEXT]                read or replace the private clipboard
//	clipimage [PATH|clear]     read (ok image N bytes | ok none) or replace the private clipboard's image with a PNG file
//	openedurl                  report the last public OpenURL request
//	openedpath                 report the last OpenPath request
//	schemehandlers [SCHEME...] declare which custom URL schemes have a registered application (none: clear)
//	openfiles PATH...|cancel   queue the next native open-dialog result
//	opendirectory PATH|cancel  queue the next native directory-dialog result
//	savefile PATH|cancel       queue the next native save-dialog result
//	quit
func Serve(session *Session, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) != "" {
			reply, quit := execute(session, line)
			if _, writeErr := io.WriteString(out, reply); writeErr != nil {
				return writeErr
			}
			if quit {
				return nil
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// execute runs one request and returns the complete reply text.
func execute(session *Session, line string) (reply string, quit bool) {
	command, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
	rest = strings.TrimLeft(rest, " ")
	var data strings.Builder
	finish := func(err error, summary string) (string, bool) {
		if err != nil {
			return data.String() + "error " + strings.ReplaceAll(err.Error(), "\n", " ") + "\n", false
		}
		return data.String() + "ok" + summary + "\n", false
	}
	// "type", "compose" and "clip" take the rest of the line verbatim.
	switch command {
	case "type":
		return finish(session.Type(rest), "")
	case "compose":
		return finish(session.Compose(rest), "")
	case "clip":
		if rest != "" {
			session.driver.SetClipboard(rest)
		}
		return finish(nil, " "+strconv.Quote(session.driver.Clipboard()))
	case "clipimage":
		switch rest {
		case "":
			image := session.driver.ClipboardImage()
			if len(image) == 0 {
				return finish(nil, " none")
			}
			return finish(nil, " image "+strconv.Itoa(len(image))+" bytes")
		case "clear":
			return finish(session.driver.SetClipboardImage(nil), "")
		}
		png, err := os.ReadFile(rest)
		if err != nil {
			return finish(err, "")
		}
		return finish(session.driver.SetClipboardImage(png), "")
	case "openedpath":
		if rest != "" {
			return finish(fmt.Errorf("openedpath takes no arguments"), "")
		}
		target, ok := session.driver.LastOpenedPath()
		if !ok {
			return finish(nil, " none")
		}
		return finish(nil, " "+strconv.Quote(target))
	case "schemehandlers":
		session.driver.SetSchemeHandlers(strings.Fields(rest)...)
		return finish(nil, "")
	case "openedurl":
		if rest != "" {
			return finish(fmt.Errorf("openedurl takes no arguments"), "")
		}
		target, ok := session.driver.LastOpenedURL()
		if !ok {
			return finish(nil, " none")
		}
		return finish(nil, " "+strconv.Quote(target))
	}
	arguments, err := splitArguments(rest)
	if err != nil {
		return finish(err, "")
	}
	numbers := func(count int) ([]float32, error) {
		if len(arguments) < count {
			return nil, fmt.Errorf("%s needs %d numbers", command, count)
		}
		values := make([]float32, count)
		for index := range values {
			value, err := strconv.ParseFloat(arguments[index], 32)
			if err != nil {
				return nil, fmt.Errorf("%s: %q is not a number", command, arguments[index])
			}
			values[index] = float32(value)
		}
		return values, nil
	}

	switch command {
	case "quit", "exit":
		return "ok bye\n", true
	case "size":
		snapshot := session.Snapshot()
		return finish(nil, fmt.Sprintf(" %dx%d scale=%g", snapshot.Width, snapshot.Height, session.options.Scale))
	case "appearance":
		if len(arguments) > 0 {
			var appearance app.Appearance
			switch arguments[0] {
			case "light":
				appearance = app.AppearanceLight
			case "dark":
				appearance = app.AppearanceDark
			case "unknown":
			default:
				return finish(fmt.Errorf("appearance is light, dark or unknown"), "")
			}
			session.SetSystemAppearance(appearance)
		}
		return finish(nil, " "+app.SystemAppearance().String())
	case "openfiles":
		if len(arguments) == 0 {
			return finish(fmt.Errorf("openfiles needs paths or cancel"), "")
		}
		if len(arguments) == 1 && arguments[0] == "cancel" {
			return finish(session.driver.QueueOpenFiles(nil, app.ErrCanceled), "")
		}
		return finish(session.driver.QueueOpenFiles(arguments, nil), "")
	case "opendirectory":
		if len(arguments) != 1 {
			return finish(fmt.Errorf("opendirectory needs one path or cancel"), "")
		}
		if arguments[0] == "cancel" {
			return finish(session.driver.QueueOpenDirectory("", app.ErrCanceled), "")
		}
		return finish(session.driver.QueueOpenDirectory(arguments[0], nil), "")
	case "savefile":
		if len(arguments) != 1 {
			return finish(fmt.Errorf("savefile needs one path or cancel"), "")
		}
		if arguments[0] == "cancel" {
			return finish(session.driver.QueueSaveFile("", app.ErrCanceled), "")
		}
		return finish(session.driver.QueueSaveFile(arguments[0], nil), "")
	case "move", "down", "up", "click", "dblclick", "rightclick":
		if command == "rightclick" && len(arguments) == 1 {
			return finish(session.RightClickLabel(arguments[0]), "")
		}
		if command == "click" && len(arguments) == 1 {
			return finish(session.ClickLabel(arguments[0]), "")
		}
		if command == "dblclick" && len(arguments) == 1 {
			return finish(session.DoubleClickLabel(arguments[0]), "")
		}
		point, err := numbers(2)
		if err != nil {
			return finish(err, "")
		}
		switch command {
		case "move":
			err = session.Move(point[0], point[1])
		case "down":
			err = session.Press(point[0], point[1])
		case "up":
			err = session.Release(point[0], point[1])
		case "click":
			err = session.Click(point[0], point[1])
		case "dblclick":
			err = session.DoubleClick(point[0], point[1])
		case "rightclick":
			err = session.RightClick(point[0], point[1])
		}
		return finish(err, "")
	case "drop", "dragover", "dragleave":
		event := ui.FileDropEvent{Phase: ui.FileDropLeave}
		if command != "dragleave" {
			if len(arguments) < 2 || command == "drop" && len(arguments) < 3 {
				return finish(fmt.Errorf("%s needs X Y%s", command, map[bool]string{true: " and at least one path"}[command == "drop"]), "")
			}
			values, err := numbers(2)
			if err != nil {
				return finish(err, "")
			}
			event = ui.FileDropEvent{Phase: ui.FileDropOver, X: values[0], Y: values[1]}
			if command == "drop" {
				event.Phase, event.Paths = ui.FileDropDrop, arguments[2:]
			}
		}
		accepted, err := session.Drop(event)
		answer := " refused"
		if accepted {
			answer = " accepted"
		}
		return finish(err, answer)
	case "scroll":
		if len(arguments) != 4 && len(arguments) != 5 {
			return finish(fmt.Errorf("scroll needs X Y DX DY and optional modifiers"), "")
		}
		values, err := numbers(4)
		if err != nil {
			return finish(err, "")
		}
		var modifiers ui.Modifiers
		if len(arguments) == 5 {
			modifiers, err = parseModifiers(arguments[4])
			if err != nil {
				return finish(err, "")
			}
		}
		return finish(session.Scroll(ui.ScrollEvent{
			Modifiers: modifiers, X: values[0], Y: values[1],
			DeltaX: values[2], DeltaY: values[3], Precise: true,
		}), "")
	case "key":
		if len(arguments) == 0 {
			return finish(fmt.Errorf("key needs at least one key"), "")
		}
		for _, spec := range arguments {
			if err := session.Keys(spec); err != nil {
				return finish(err, "")
			}
		}
		return finish(nil, "")
	case "keydown", "keyup":
		if len(arguments) != 1 {
			return finish(fmt.Errorf("%s needs one key", command), "")
		}
		if command == "keydown" {
			return finish(session.KeyDown(arguments[0]), "")
		}
		return finish(session.KeyUp(arguments[0]), "")
	case "blur":
		return finish(session.LoseFocus(), "")
	case "select":
		span, err := numbers(2)
		if err != nil {
			return finish(err, "")
		}
		return finish(session.driver.SelectText(int(span[0]), int(span[1])), "")
	case "resize":
		size, err := numbers(2)
		if err != nil {
			return finish(err, "")
		}
		return finish(session.Resize(int(size[0]), int(size[1])), "")
	case "frame":
		frame := session.driver.Frame()
		return finish(nil, fmt.Sprintf(" x=%g y=%g w=%g h=%g", frame.X, frame.Y, frame.Width, frame.Height))
	case "windowmove":
		point, err := numbers(2)
		if err != nil {
			return finish(err, "")
		}
		return finish(session.driver.MoveWindow(float32(point[0]), float32(point[1])), "")
	case "minimize":
		switch rest {
		case "on":
			return finish(session.driver.SetMinimized(true), "")
		case "off":
			return finish(session.driver.SetMinimized(false), "")
		default:
			return finish(fmt.Errorf("minimize takes on or off"), "")
		}
	case "maximize":
		switch rest {
		case "on":
			return finish(session.driver.SetMaximized(true), "")
		case "off":
			return finish(session.driver.SetMaximized(false), "")
		default:
			return finish(fmt.Errorf("maximize takes on or off"), "")
		}
	case "wait":
		milliseconds, err := numbers(1)
		if err != nil {
			return finish(err, "")
		}
		return finish(session.Wait(time.Duration(milliseconds[0])*time.Millisecond), "")
	case "settle":
		quiet := 100 * time.Millisecond
		if len(arguments) > 0 {
			milliseconds, err := numbers(1)
			if err != nil {
				return finish(err, "")
			}
			quiet = time.Duration(milliseconds[0]) * time.Millisecond
		}
		return finish(session.Settle(quiet, 10*time.Second), "")
	case "waitfor":
		if len(arguments) == 0 {
			return finish(fmt.Errorf("waitfor needs the text to wait for"), "")
		}
		timeout := 5 * time.Second
		if len(arguments) > 1 {
			milliseconds, err := strconv.Atoi(arguments[1])
			if err != nil {
				return finish(fmt.Errorf("waitfor: %q is not a number", arguments[1]), "")
			}
			timeout = time.Duration(milliseconds) * time.Millisecond
		}
		err := session.WaitFor(func(snapshot app.Snapshot) bool { return snapshotHasText(snapshot, arguments[0]) }, timeout)
		if err != nil {
			err = fmt.Errorf("%q did not appear within %v; screen has %q", arguments[0], timeout, summarizeTexts(session.Snapshot()))
		}
		return finish(err, "")
	case "shot":
		if len(arguments) < 1 || len(arguments) > 2 || len(arguments) == 2 && arguments[1] != "now" {
			return finish(fmt.Errorf("shot needs a path, and now to skip the rest"), "")
		}
		if len(arguments) == 1 {
			if err := session.Rest(); err != nil {
				return finish(err, "")
			}
		}
		err := session.Screenshot(arguments[0])
		return finish(err, " "+arguments[0])
	case "record":
		if len(arguments) < 2 || len(arguments) > 3 {
			return finish(fmt.Errorf("record needs a directory and a duration in ms, and optionally fps"), "")
		}
		milliseconds, err := strconv.ParseFloat(arguments[1], 64)
		if err != nil {
			return finish(fmt.Errorf("record: %q is not a number", arguments[1]), "")
		}
		options := RecordOptions{Duration: time.Duration(milliseconds * float64(time.Millisecond))}
		if len(arguments) == 3 {
			fps, err := strconv.Atoi(arguments[2])
			if err != nil {
				return finish(fmt.Errorf("record: fps %q is not a whole number", arguments[2]), "")
			}
			options.FPS = fps
		}
		frames, err := session.Record(arguments[0], options)
		if frames == 1 {
			return finish(err, " 1 frame")
		}
		return finish(err, fmt.Sprintf(" %d frames", frames))
	case "find":
		if len(arguments) != 1 {
			return finish(fmt.Errorf("find needs one label"), "")
		}
		hit, found := session.Hit(arguments[0])
		if !found {
			return finish(fmt.Errorf("no control labelled %q", arguments[0]), "")
		}
		return finish(nil, " "+describeHit(session.Snapshot().Tree, hit))
	case "hits":
		filter := strings.ToLower(strings.Join(arguments, " "))
		for _, hit := range session.Snapshot().Hits {
			if hit.Block || hit.MovesWindow || filter != "" && !strings.Contains(strings.ToLower(hit.Label), filter) {
				continue
			}
			data.WriteString(describeHit(session.Snapshot().Tree, hit) + "\n")
		}
		return finish(nil, "")
	case "texts":
		filter := strings.ToLower(strings.Join(arguments, " "))
		for _, painted := range session.Snapshot().Texts {
			if filter != "" && !strings.Contains(strings.ToLower(painted.Value), filter) {
				continue
			}
			fmt.Fprintf(&data, "x=%g y=%g w=%g h=%g size=%g %q\n", painted.Bounds.X, painted.Bounds.Y, painted.Bounds.Width, painted.Bounds.Height, painted.Style.Size, painted.Value)
		}
		return finish(nil, "")
	case "layout":
		problems := CheckLayout(session.Snapshot())
		for _, problem := range problems {
			fmt.Fprintf(&data, "%s x=%g y=%g w=%g h=%g %q %s\n", problem.Kind,
				problem.Bounds.X, problem.Bounds.Y, problem.Bounds.Width, problem.Bounds.Height, problem.Text, problem.Detail)
		}
		return finish(nil, fmt.Sprintf(" %d problems", len(problems)))
	case "focus":
		snapshot := session.Snapshot()
		label := ""
		for _, hit := range snapshot.Hits {
			if hit.ID == snapshot.Focused && !snapshot.Focused.IsZero() {
				label = hit.Label
			}
		}
		return finish(nil, fmt.Sprintf(" id=%s label=%q", snapshot.Tree.Text(snapshot.Focused), label))
	case "editor":
		editor := session.Snapshot().Editor
		if editor == nil {
			return finish(nil, " none")
		}
		value := editor.Value
		if editor.Secure {
			value = strings.Repeat("•", utf8.RuneCountInString(value))
		}
		return finish(nil, fmt.Sprintf(" value=%q selection=%d,%d secure=%t", value, editor.Selection[0], editor.Selection[1], editor.Secure))
	}
	return finish(fmt.Errorf("unknown command %q", command), "")
}

func describeHit(tree *ui.Tree, hit ui.Hit) string {
	return fmt.Sprintf("id=%s x=%g y=%g w=%g h=%g label=%q", tree.Text(hit.ID), hit.Bounds.X, hit.Bounds.Y, hit.Bounds.Width, hit.Bounds.Height, hit.Label)
}

func snapshotHasText(snapshot app.Snapshot, substring string) bool {
	for _, painted := range snapshot.Texts {
		if strings.Contains(painted.Value, substring) {
			return true
		}
	}
	return false
}

// summarizeTexts lists the first painted texts, so a failed wait says what was
// on screen instead.
func summarizeTexts(snapshot app.Snapshot) string {
	var values []string
	for _, painted := range snapshot.Texts {
		if painted.Value != "" {
			values = append(values, painted.Value)
		}
		if len(values) == 12 {
			break
		}
	}
	return strings.Join(values, " | ")
}

// splitArguments splits a request tail on spaces, honoring double quotes.
func splitArguments(text string) ([]string, error) {
	var arguments []string
	var current strings.Builder
	inQuotes, started := false, false
	runes := []rune(text)
	for index := 0; index < len(runes); index++ {
		r := runes[index]
		switch {
		case inQuotes && r == '\\' && index+1 < len(runes):
			index++
			current.WriteRune(runes[index])
		case r == '"':
			inQuotes, started = !inQuotes, true
		case r == ' ' && !inQuotes:
			if started {
				arguments = append(arguments, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if inQuotes {
		return nil, fmt.Errorf("unterminated quote in %q", text)
	}
	if started {
		arguments = append(arguments, current.String())
	}
	return arguments, nil
}

// Run is the app.UseRunner implementation behind headless/auto: it opens a
// Session for config and serves the line protocol on standard input and
// output instead of opening a window. VIVA_HEADLESS_SCALE sets the screenshot
// scale (default 2).
func Run(config app.Config, canvas *ui.Canvas) error {
	options := Options{}
	if value := os.Getenv("VIVA_HEADLESS_SCALE"); value != "" {
		scale, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("headless: VIVA_HEADLESS_SCALE=%q is not a number", value)
		}
		options.Scale = scale
	}
	session, err := Open(config, canvas, options)
	if err != nil {
		return err
	}
	defer session.Close()
	snapshot := session.Snapshot()
	fmt.Fprintf(os.Stdout, "ok ready %dx%d scale=%g\n", snapshot.Width, snapshot.Height, session.options.Scale)
	return Serve(session, os.Stdin, os.Stdout)
}
