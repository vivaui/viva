// Package upload is the opt-in page control for choosing files to send: an area
// that takes files dragged from the system or chosen with the system's picker,
// and the list of the files chosen so far, each with its progress and a button
// to remove it. A page keeps an Upload in its model beside the list:
//
//	m.upload = upload.New()
//	// in the page:
//	m.upload.View(c, "Attachments", upload.Spec{
//		Items: m.files, Multiple: true, Accept: []string{".png", ".jpg"},
//	}).FillWidth()
//	for _, file := range m.upload.Added() {
//		m.files = append(m.files, upload.Item{Name: file.Name(), Size: file.Size()})
//		// start sending file; set the item's Status and Progress as it goes
//	}
//	if i, ok := m.upload.Removed(); ok {
//		m.files = slices.Delete(m.files, i, i+1) // and stop sending it
//	}
//
// The control sends nothing: the page owns the files and the transfer, and says
// how far each Item is (Status, Progress, Error). The control reports what the
// user did: Added the files that passed the checks, Rejected the ones that did
// not, Removed and Retried the item of a button pressed.
//
// A file is added when it is a file and not a folder, its name ends in one of
// Accept when that is set, and, unless Multiple is set, it is the first of its
// batch. With Limit set, a batch is cut where the list would be longer. A
// browser gives files without paths; ui.FilePath says whether a file has one.
//
// The picker is the Choose button, which is the keyboard's way to the picker
// too. Dropping on the area adds the same way.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an Upload in the model: its state and handlers are made
// once, so a warm paint allocates nothing. Do not copy an Upload after it has
// been declared.
package upload

import (
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	uploadIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(21, 15), gfx.LineTo(21, 19), gfx.QuadTo(21, 21, 19, 21), gfx.LineTo(5, 21), gfx.QuadTo(3, 21, 3, 19), gfx.LineTo(3, 15),
		gfx.MoveTo(17, 8), gfx.LineTo(12, 3), gfx.LineTo(7, 8),
		gfx.MoveTo(12, 3), gfx.LineTo(12, 15),
	}}
	fileIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(14.5, 2), gfx.LineTo(6, 2), gfx.QuadTo(4, 2, 4, 4), gfx.LineTo(4, 20), gfx.QuadTo(4, 22, 6, 22), gfx.LineTo(18, 22),
		gfx.QuadTo(20, 22, 20, 20), gfx.LineTo(20, 7.5), gfx.LineTo(14.5, 2),
		gfx.MoveTo(14, 2), gfx.LineTo(14, 8), gfx.LineTo(20, 8),
	}}
	removeIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(18, 6), gfx.LineTo(6, 18), gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
	}}
	retryIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(21, 12), gfx.CubicTo(21, 16.97, 16.97, 21, 12, 21), gfx.CubicTo(9.5, 21, 7.24, 19.98, 5.6, 18.3),
		gfx.MoveTo(3, 12), gfx.CubicTo(3, 7.03, 7.03, 3, 12, 3), gfx.CubicTo(14.5, 3, 16.76, 4.02, 18.4, 5.7),
		gfx.MoveTo(21, 3), gfx.LineTo(21, 8), gfx.LineTo(16, 8),
		gfx.MoveTo(3, 21), gfx.LineTo(3, 16), gfx.LineTo(8, 16),
	}}
)

// Status is how far the sending of an item has got.
type Status uint8

const (
	// Pending is an item chosen and not yet sent: the zero value.
	Pending Status = iota
	// Uploading is an item being sent; Progress says how far.
	Uploading
	// Done is an item that was sent.
	Done
	// Failed is an item that could not be sent; it offers Retry.
	Failed
)

// Item is one file of the list, as the page keeps it.
type Item struct {
	Name string
	// Size is the file's length in bytes; not shown when zero or less.
	Size   int64
	Status Status
	// Progress is the fraction, 0 to 1, of an Uploading item that was sent.
	Progress float32
	// Error is the words that say why a Failed item failed, in place of its size.
	Error string
}

// Spec declares the area and the list.
type Spec struct {
	// Items are the files chosen so far, in the order the list shows them.
	Items []Item
	// Title is the line in the area, "Drag files here" when empty; Hint is a muted
	// line under it, for what the area accepts ("PNG or JPEG, up to 5 MB"), and is
	// not drawn when empty. Choose is the picker button's label, "Choose files"
	// when empty.
	Title, Hint, Choose string
	// Accept are the file name endings the area takes, with the dot (".png"),
	// matched without regard to case. Empty takes every file. AcceptName names
	// them in the picker, "Supported files" when empty.
	Accept     []string
	AcceptName string
	// Multiple lets one batch add several files; otherwise only the first is added.
	Multiple bool
	// Limit is how many items the list may have; zero is no limit.
	Limit int
	// Disabled leaves the area and the buttons inert.
	Disabled bool
	// RemoveLabel and RetryLabel are the accessible names of the buttons of a row,
	// before the file's name: "Remove" and "Retry" when empty.
	RemoveLabel, RetryLabel string
}

// Upload is one area and its list.
type Upload struct {
	drop ui.FileDropState

	added, rejected []gfx.FileDropFile
	removed         int
	retried         int
	picked          []gfx.FileDropFile
	pickedFn        func([]gfx.FileDropFile, error)
	rows            []rowText
}

// rowText is the words of a row that are made from the item and kept while it
// does not change.
type rowText struct {
	name   string
	size   int64
	sizeAt string
	remove string
	retry  string
	labels [2]string // the button labels the words were made with
}

// New returns an empty upload.
func New() *Upload {
	u := &Upload{removed: -1, retried: -1}
	u.pickedFn = u.onPicked
	return u
}

// Added are the files the user chose or dropped in the paint just made that
// passed the checks, in order; the page adds them to its list and starts sending
// them.
func (u *Upload) Added() []gfx.FileDropFile { return u.added }

// Rejected are the files chosen or dropped in the paint just made that were not
// added: a folder, a file whose name is not in Accept, or one past Limit or past
// the first of a single-file batch.
func (u *Upload) Rejected() []gfx.FileDropFile { return u.rejected }

// Removed reports the index in Spec.Items of the item whose remove button was
// pressed in the paint just made.
func (u *Upload) Removed() (index int, ok bool) { return u.removed, u.removed >= 0 }

// Retried reports the index in Spec.Items of the failed item whose retry button
// was pressed in the paint just made.
func (u *Upload) Retried() (index int, ok bool) { return u.retried, u.retried >= 0 }

func (u *Upload) onPicked(files []gfx.FileDropFile, err error) {
	if err == nil {
		u.picked = files
	}
}

// View declares the area and the list at c. label names the area for assistive
// technology. The control is as wide as the room it is given and as tall as its
// content; give it FillWidth. Declare the view on its own, not inside a Memo.
func (u *Upload) View(c ui.C, label string, spec Spec) ui.Box {
	if u == nil {
		panic("upload: View needs its Upload")
	}
	u.added, u.rejected = u.added[:0], u.rejected[:0]
	u.removed, u.retried = -1, -1
	if files, _ := u.drop.Dropped(); len(files) > 0 {
		u.take(files, spec)
	}
	if len(u.picked) > 0 {
		u.take(u.picked, spec)
		u.picked = nil
	}
	theme := c.Theme()
	root := ui.Column(c).Gap(theme.Metrics.GapItem).Label(label)
	root.Children(func() {
		u.area(c, spec)
		if len(spec.Items) > 0 {
			u.list(c, spec)
		}
	})
	return root
}

// take checks a batch of files against the spec and sorts them into Added and
// Rejected.
func (u *Upload) take(files []gfx.FileDropFile, spec Spec) {
	if spec.Disabled {
		return
	}
	room := len(files)
	if spec.Limit > 0 {
		room = max(spec.Limit-len(spec.Items)-len(u.added), 0)
	}
	if !spec.Multiple {
		room = min(room, 1-len(u.added))
	}
	for _, file := range files {
		if file.IsDir() || !accepts(spec.Accept, file.Name()) || room <= 0 {
			u.rejected = append(u.rejected, file)
			continue
		}
		u.added = append(u.added, file)
		room--
	}
}

// accepts reports whether the name ends in one of the endings, or there are none.
func accepts(endings []string, name string) bool {
	if len(endings) == 0 {
		return true
	}
	for _, ending := range endings {
		if len(name) >= len(ending) && strings.EqualFold(name[len(name)-len(ending):], ending) {
			return true
		}
	}
	return false
}

// area declares the place files are dropped on, with the picker's button.
func (u *Upload) area(c ui.C, spec Spec) {
	theme := c.Theme()
	metrics := theme.Metrics
	title, choose := spec.Title, spec.Choose
	if title == "" {
		title = "Drag files here"
	}
	if choose == "" {
		choose = "Choose files"
	}
	box := ui.Column(c).FillWidth().Center().Gap(metrics.GapItem).Pad(metrics.PadSurface * 1.5)
	box = box.Radius(metrics.RadiusSurface).Border(1, theme.Border).Dashed()
	if !spec.Disabled {
		box = box.DropFiles(&u.drop)
		if u.drop.Over() {
			tint := theme.Primary
			tint.Alpha *= 0.08
			box = box.Background(tint).Border(1, theme.Primary)
		}
	}
	box.Children(func() {
		ink := theme.MutedForeground
		ui.Icon(c, uploadIcon).Size(24, 24).Color(ink)
		ui.Text(c, title)
		if spec.Hint != "" {
			ui.Text(c, spec.Hint).FontSize(metrics.FontSizeS).Muted()
		}
		if ui.Button(c, choose).Outline().Small().Disabled(spec.Disabled).Clicked() {
			u.choose(c, spec)
		}
	})
}

// choose opens the picker. A native picker answers at once, so its files are
// checked in this paint; a browser's arrive in a later one.
func (u *Upload) choose(c ui.C, spec Spec) {
	options := ui.OpenFileOptions{AllowMultiple: spec.Multiple}
	if len(spec.Accept) > 0 {
		name := spec.AcceptName
		if name == "" {
			name = "Supported files"
		}
		var pattern strings.Builder
		for i, ending := range spec.Accept {
			if i > 0 {
				pattern.WriteByte(';')
			}
			pattern.WriteByte('*')
			pattern.WriteString(ending)
		}
		options.Filters = []ui.FileFilter{{Name: name, Pattern: pattern.String()}}
	}
	if err := ui.OpenFiles(c, options, u.pickedFn); err != nil {
		return
	}
	if len(u.picked) > 0 {
		u.take(u.picked, spec)
		u.picked = nil
	}
}

// list declares one row for each item.
func (u *Upload) list(c ui.C, spec Spec) {
	theme := c.Theme()
	metrics := theme.Metrics
	for len(u.rows) < len(spec.Items) {
		u.rows = append(u.rows, rowText{})
	}
	removeLabel, retryLabel := spec.RemoveLabel, spec.RetryLabel
	if removeLabel == "" {
		removeLabel = "Remove"
	}
	if retryLabel == "" {
		retryLabel = "Retry"
	}
	ui.Column(c).FillWidth().Gap(metrics.GapItem).Children(func() {
		for i := range spec.Items {
			item := &spec.Items[i]
			text := u.rows[i].refresh(item, removeLabel, retryLabel)
			row := c.KeyInt(i)
			ink := theme.MutedForeground
			switch item.Status {
			case Done:
				ink = theme.Success
			case Failed:
				ink = theme.Danger
			}
			ui.Column(row).FillWidth().Gap(4).Pad(metrics.GapItem).Radius(metrics.RadiusControl).Border(1, theme.Border).Children(func() {
				ui.Row(row).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
					ui.Icon(row, fileIcon).Size(16, 16).Color(ink).Shrink(0)
					ui.Text(row, item.Name).Grow(1).Basis(0).Ellipsis()
					meta := text.sizeAt
					if item.Status == Failed && item.Error != "" {
						meta = item.Error
					}
					if meta != "" {
						note := ui.Text(row, meta).FontSize(metrics.FontSizeS).Shrink(0)
						if item.Status == Failed {
							note.Color(theme.Danger)
						} else {
							note.Muted()
						}
					}
					if item.Status == Failed {
						if ui.Button(row, text.retry).Icon(retryIcon).IconOnly().Ghost().Disabled(spec.Disabled).Shrink(0).Clicked() {
							u.retried = i
						}
					}
					if ui.Button(row, text.remove).Icon(removeIcon).IconOnly().Ghost().Disabled(spec.Disabled).Shrink(0).Clicked() {
						u.removed = i
					}
				})
				if item.Status == Uploading {
					ui.Progress(row, item.Progress).FillWidth().Height(4)
				}
			})
		}
	})
}

// refresh remakes the words of a row that depend on the item when they changed.
func (r *rowText) refresh(item *Item, removeLabel, retryLabel string) *rowText {
	if r.size != item.Size || r.sizeAt == "" && item.Size > 0 {
		r.size, r.sizeAt = item.Size, ""
		if item.Size > 0 {
			r.sizeAt = formatSize(item.Size)
		}
	}
	if r.name != item.Name || r.labels != [2]string{removeLabel, retryLabel} {
		r.name, r.labels = item.Name, [2]string{removeLabel, retryLabel}
		r.remove = removeLabel + " " + item.Name
		r.retry = retryLabel + " " + item.Name
	}
	return r
}

// formatSize writes a length in bytes in the largest unit that keeps it above
// one, with one decimal for kilobytes and up.
func formatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	value, suffix := float64(n), ""
	for _, s := range [...]string{"KB", "MB", "GB", "TB"} {
		value /= unit
		suffix = s
		if value < unit {
			break
		}
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + suffix
}
