//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"time"
)

// RecordOptions configures a recording.
type RecordOptions struct {
	// Duration is how much animation time the recording covers, starting at the
	// current frame. It is the session's virtual clock, the one Wait moves, so
	// the same script records the same frames on every run. Record a little
	// longer than a transition to include its end: a frame is written for each
	// interval it is on screen, not for the instant the window closes.
	Duration time.Duration
	// ShowInput draws the input the session delivered lately over the frames: a
	// growing, fading ring where the pointer pressed, and a chip at the bottom of
	// the window naming the keys or text typed. It shows a click made just before
	// Record, so a script reads as it is done: click, then record what follows.
	// Without it the frames are the window's own pixels and nothing else.
	ShowInput bool
	// FPS is how many frames a second the recording samples, at most 120: that
	// is the cadence the driver paints animating frames at, so a faster rate
	// would only repeat them. Zero means 60.
	FPS int
}

const (
	defaultRecordFPS   = 60
	maxRecordFPS       = 120
	recordManifestName = "manifest.json"
)

// recordManifest is manifest.json: the PNG files beside it, in order, and how
// long each is on screen. Frames that repeated are folded into the one before
// them, so the durations, not a fixed frame rate, are what plays the recording
// at the speed it happened.
type recordManifest struct {
	Version    int           `json:"version"`
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	Scale      float64       `json:"scale"`
	FPS        int           `json:"fps"`
	DurationNS int64         `json:"duration_ns"`
	Frames     []recordFrame `json:"frames"`
}

type recordFrame struct {
	File       string `json:"file"`
	DurationNS int64  `json:"duration_ns"`
}

// Record plays options.Duration of animation and writes it to dir as a PNG
// sequence, frame-0000.png onwards, plus a manifest.json that says how long each
// frame is on screen. A frame whose pixels did not change is written once and
// its duration grows instead, so an application at rest records as one still
// frame rather than hundreds, and a recording costs disk only where something
// moved. The number of PNG files written is returned; when Record fails partway
// that is how many were written before the error.
//
// Recording needs the offscreen renderer Image needs, and returns
// ErrNoRenderer without one. dir is created when it does not exist; use a fresh
// directory per recording, because a recording shorter than an earlier one in
// the same directory leaves that recording's frames behind (the manifest lists
// the ones this one wrote).
//
// The manifest is:
//
//	{
//	  "version": 1,
//	  "width": 1400, "height": 900, "scale": 2, "fps": 60,
//	  "duration_ns": 700000000,
//	  "frames": [{"file": "frame-0000.png", "duration_ns": 16666666}, ...]
//	}
//
// width and height are logical pixels; the PNG files are scale times that. The
// durations sum to duration_ns. ffmpeg assembles them with a concat script
// (file frame-0000.png, duration 0.016666666, and so on), or the frames can be
// reviewed one by one.
func (session *Session) Record(dir string, options RecordOptions) (int, error) {
	if options.Duration <= 0 {
		return 0, fmt.Errorf("headless: record duration must be positive, got %v", options.Duration)
	}
	fps := options.FPS
	if fps == 0 {
		fps = defaultRecordFPS
	}
	if fps < 1 || fps > maxRecordFPS {
		return 0, fmt.Errorf("headless: record fps must be between 1 and %d, got %d", maxRecordFPS, options.FPS)
	}
	session.showInput = options.ShowInput
	defer func() { session.showInput = false }()
	first, err := session.Image()
	if err != nil {
		return 0, err
	}
	width, height := first.Bounds().Dx(), first.Bounds().Dy()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	interval := time.Second / time.Duration(fps)
	slots := int(options.Duration / interval)
	if slots < 1 {
		slots = 1
	}
	frames := make([]recordFrame, 0, slots)
	var last []byte
	written := 0
	// capture writes one sampled frame, or lengthens the one it repeats.
	capture := func(picture *image.RGBA, shown time.Duration) error {
		if last != nil && bytes.Equal(last, picture.Pix) {
			frames[len(frames)-1].DurationNS += int64(shown)
			return nil
		}
		name := fmt.Sprintf("frame-%04d.png", written)
		if err := writePNG(filepath.Join(dir, name), picture); err != nil {
			return err
		}
		last = picture.Pix
		frames = append(frames, recordFrame{File: name, DurationNS: int64(shown)})
		written++
		return nil
	}
	for slot := range slots {
		shown := interval
		if slot == slots-1 {
			// The last slot absorbs the remainder the frame rate does not divide.
			shown = options.Duration - time.Duration(slots-1)*interval
		}
		picture := first
		if slot > 0 {
			if err := session.driver.Advance(interval); err != nil {
				return written, err
			}
			if picture, err = session.Image(); err != nil {
				return written, err
			}
		}
		// A manifest describes one frame size; a window that resized mid-recording
		// has no single sequence to write.
		if picture.Bounds().Dx() != width || picture.Bounds().Dy() != height {
			return written, fmt.Errorf("headless: the window changed size while recording")
		}
		if err := capture(picture, shown); err != nil {
			return written, err
		}
	}
	snapshot := session.driver.Snapshot()
	manifest := recordManifest{
		Version:    1,
		Width:      snapshot.Width,
		Height:     snapshot.Height,
		Scale:      session.options.Scale,
		FPS:        fps,
		DurationNS: int64(options.Duration),
		Frames:     frames,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return written, err
	}
	if err := os.WriteFile(filepath.Join(dir, recordManifestName), append(data, '\n'), 0o644); err != nil {
		return written, err
	}
	return written, nil
}
