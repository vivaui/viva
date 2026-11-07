package components

import (
	"encoding/csv"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestTableUsesHorizontalRulesAndOfficialRowStates(t *testing.T) {
	theme := ui.LightTheme()
	columns := []TableColumn{{Width: 120}, {Width: 90, Alignment: TableAlignCenter}, {Alignment: TableAlignRight}}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, ui.Rect{Width: 320, Height: 140}, columns)
	table.Header([]string{"Name", "State", "Total"})
	table.RowState(ui.Item(7), []string{"Ada", "Active", "$42"}, TableRowState{Expanded: true})
	table.Footer([]string{"", "Total", "$42"})
	table.Caption("Current members")
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].Label != "Ada" {
		t.Fatalf("table hits = %+v", canvas.Hits())
	}
	if canvas.Texts()[5].Bounds.X <= 210 || !canvas.Texts()[0].HasClip {
		t.Fatalf("table cell placement = %+v", canvas.Texts())
	}
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Width == 1 && rectangle.Bounds.Height > 1 {
			t.Fatal("table emitted a vertical cell border")
		}
	}
	if canvas.Texts()[len(canvas.Texts())-1].Value != "Current members" {
		t.Fatal("table caption missing")
	}
}

func TestTableFontSizeAndCellPaddingOverrideBodyAndHeader(t *testing.T) {
	theme := ui.LightTheme()
	const size, padding = float32(12.5), float32(12)
	columns := []TableColumn{{Width: 160}}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, ui.Rect{Width: 160, Height: 120}, columns).
		FontSize(size).CellPadding(padding)
	table.Header([]string{"Heading"})
	table.Row(ui.Item(1), []string{"Value"}, false)

	texts := canvas.Texts()
	if len(texts) != 2 {
		t.Fatalf("table painted %d texts, want the header and body cell: %+v", len(texts), texts)
	}
	for _, text := range texts {
		if text.Style.Size != size {
			t.Errorf("%q is painted at %v px, want %v", text.Value, text.Style.Size, size)
		}
		if text.Bounds.X != padding {
			t.Errorf("%q starts at x=%v, want the cell padding %v", text.Value, text.Bounds.X, padding)
		}
		if want := columns[0].Width - 2*padding; text.Bounds.Width != want {
			t.Errorf("%q has %v px of room, want %v", text.Value, text.Bounds.Width, want)
		}
	}
}

func TestTableTypographyDefaultsToTheTheme(t *testing.T) {
	theme := ui.DarkTheme()
	columns := []TableColumn{{Width: 160}}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, ui.Rect{Width: 160, Height: 120}, columns).
		FontSize(0).CellPadding(0)
	table.Header([]string{"Heading"})
	table.Row(ui.Item(1), []string{"Value"}, false)

	texts := canvas.Texts()
	if len(texts) != 2 {
		t.Fatalf("table painted %d texts, want the header and body cell: %+v", len(texts), texts)
	}
	for _, text := range texts {
		if text.Style.Size != theme.Metrics.FontSizeM {
			t.Errorf("%q is painted at %v px, want the theme's %v", text.Value, text.Style.Size, theme.Metrics.FontSizeM)
		}
		if text.Bounds.X != theme.Metrics.GapItem {
			t.Errorf("%q starts at x=%v, want the theme's %v", text.Value, text.Bounds.X, theme.Metrics.GapItem)
		}
		if want := columns[0].Width - 2*theme.Metrics.GapItem; text.Bounds.Width != want {
			t.Errorf("%q has %v px of room, want %v", text.Value, text.Bounds.Width, want)
		}
	}
}

func TestTableFontSizeAndCellPaddingOverrideSortableHeader(t *testing.T) {
	theme := ui.LightTheme()
	const size, padding = float32(12.5), float32(12)
	columns := []TableColumn{{Width: 160}}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, ui.Rect{Width: 160, Height: 120}, columns).
		FontSize(size).CellPadding(padding)
	table.SortableHeader([]TableHeader{{Label: "Heading"}})

	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("sortable header painted %d texts, want one: %+v", len(texts), texts)
	}
	if texts[0].Style.Size != size || texts[0].Bounds.X != padding {
		t.Fatalf("sortable header text is %+v, want size %v at x=%v", texts[0], size, padding)
	}
	if want := columns[0].Width - 2*padding; texts[0].Bounds.Width != want {
		t.Fatalf("sortable header has %v px of room, want %v", texts[0].Bounds.Width, want)
	}
}

func TestTableCommandStreamIsStable(t *testing.T) {
	columns := []TableColumn{{Width: 100}, {Width: 0}}
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{Width: 240, Height: 100}, columns)
		table.Header([]string{"Key", "Value"})
		table.Row(ui.Item(1), []string{"Region", "Europe"}, true)
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical tables produced different command streams")
	}
}

func TestTableRowLayoutReturnsReusableCellGeometry(t *testing.T) {
	columns := []TableColumn{{Width: 80}, {Width: 120, Inset: 22}, {Width: 0}}
	storage := make([]ui.Rect, 0, len(columns))
	var canvas ui.Canvas
	table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{X: 10, Y: 20, Width: 300, Height: 80}, columns)
	table.Header([]string{"", "Name", "Size"})
	layout := table.RowLayout(ui.Item(7), []string{"", "notes.txt", "12 B"}, TableRowState{Selected: true}, storage)
	if layout.Bounds != (ui.Rect{X: 10, Y: 20 + TableHeaderHeight(ui.DarkTheme()), Width: 300, Height: TableRowHeight(ui.DarkTheme())}) || len(layout.Cells) != 3 {
		t.Fatalf("row layout = %+v", layout)
	}
	if layout.Cells[0].Width != 80 || layout.Cells[1].X != 90 || layout.Cells[1].Width != 120 || layout.Cells[2].Width != 100 {
		t.Fatalf("cell geometry = %+v", layout.Cells)
	}
	// A cell without text paints nothing, so the name is the last text of the row.
	texts := canvas.Texts()
	if name := texts[len(texts)-2]; name.Value != "notes.txt" || name.Bounds.X != 90+8+22 {
		t.Fatalf("inset name = %+v", name)
	}
	if &layout.Cells[:1][0] != &storage[:1][0] {
		t.Fatal("row layout did not reuse caller storage")
	}
}

func TestTableEndsCellsThatDoNotFitInAnEllipsis(t *testing.T) {
	theme := ui.DarkTheme()
	columns := []TableColumn{{Width: 70}, {Width: 70, Alignment: TableAlignRight}, {Width: 0}}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, ui.Rect{Width: 300, Height: 120}, columns)
	table.Header([]string{"Marker / hash", "Key", "Comment"})
	table.Row(ui.Item(1), []string{"@cert-authority", "ssh-ed25519", "short"}, false)
	texts := canvas.Texts()
	if len(texts) != 6 {
		t.Fatalf("texts = %+v", texts)
	}
	room := columns[0].Width - theme.Metrics.GapItem*2
	for _, index := range []int{0, 3, 4} {
		text := texts[index]
		if !strings.HasSuffix(text.Value, "…") || canvas.TextWidthWeight(text.Value, text.Style.Size, text.Style.Weight) > text.Bounds.Width {
			t.Errorf("cell %d = %q (width %v), want it ended in an ellipsis within its cell", index, text.Value, canvas.TextWidthWeight(text.Value, text.Style.Size, text.Style.Weight))
		}
	}
	if texts[0].Bounds.Width != room {
		t.Errorf("ellipsized cell bounds = %v, want %v", texts[0].Bounds.Width, room)
	}
	if texts[1].Value != "Key" || texts[2].Value != "Comment" || texts[5].Value != "short" {
		t.Errorf("a cell that fits was changed: %+v", texts)
	}
}

func TestTableCopySelectionUsesCompleteTruncatedCells(t *testing.T) {
	theme := ui.DarkTheme()
	longValue := strings.Repeat("long-environment-value-", 12)
	rows := [][]string{
		{"LONG", longValue},
		{"SHORT", "value"},
	}
	columns := []TableColumn{{Width: 100}, {Width: 160}}
	expected := "Variable\tValue\nLONG\t" + longValue + "\nSHORT\tvalue"
	bounds := ui.Rect{X: 20, Y: 30, Width: 260, Height: 200}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, bounds, columns)
	table.Selection = TableSelection{
		Enabled: true,
		Anchor:  0,
		Focus:   len(expected),
	}
	table.Header([]string{"Variable", "Value"})
	for index, row := range rows {
		table.Row(ui.Item(9+index), row, false)
	}

	if table.TextLen() != len(expected) {
		t.Fatalf("text length = %d, want %d", table.TextLen(), len(expected))
	}
	if got := table.SelectedText(table.Selection); got != expected {
		t.Fatalf("selected text = %q, want %q", got, expected)
	}
	if hits := canvas.Hits(); len(hits) != 2 || hits[0].ID != ui.Item(9) || hits[1].ID != ui.Item(10) {
		t.Fatalf("table hits = %+v", hits)
	}
	selectedBackground := false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == theme.Selection {
			selectedBackground = true
			break
		}
	}
	if !selectedBackground {
		t.Fatal("selection did not paint using the semantic selection color")
	}
	if !strings.Contains(strings.Join(tableTextValues(canvas.Texts()), " "), "…") {
		t.Fatal("long cell was not visually truncated")
	}
}

func TestTableCopySelectionMapsPointerToCellText(t *testing.T) {
	theme := ui.DarkTheme()
	bounds := ui.Rect{X: 10, Y: 20, Width: 300, Height: 180}
	columns := []TableColumn{{Width: 100}, {Width: 0}}
	rows := [][]string{{"FIRST", strings.Repeat("x", 80)}, {"SECOND", "tail"}}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, bounds, columns)
	table.Selection.Enabled = true
	table.Header([]string{"Variable", "Value"})
	for _, row := range rows {
		table.Row(ui.ID{}, row, false)
	}

	firstRowY := bounds.Y + TableHeaderHeight(theme) + 2
	offset, ok := table.OffsetAt(ui.Point{X: bounds.X + theme.Metrics.GapItem, Y: firstRowY})
	firstRowStart := len("Variable\tValue\n")
	secondRowStart := len("Variable\tValue\nFIRST\t" + strings.Repeat("x", 80) + "\n")
	if !ok || offset < firstRowStart || offset >= secondRowStart {
		t.Fatalf("first cell offset = %d, %v", offset, ok)
	}
	lastRowY := bounds.Y + TableHeaderHeight(theme) + TableRowHeight(theme) + 2
	lastValueStart := len("Variable\tValue\nFIRST\t" + strings.Repeat("x", 80) + "\nSECOND\t")
	offset, ok = table.OffsetAt(ui.Point{
		X: bounds.X + columns[0].Width + theme.Metrics.GapItem + 1,
		Y: lastRowY,
	})
	if !ok || offset < lastValueStart || offset > table.TextLen() {
		t.Fatalf("last cell offset = %d, want within %d..%d (%v)",
			offset, lastValueStart, table.TextLen(), ok)
	}
}

func TestTableCopySelectionClampsReversedRange(t *testing.T) {
	var canvas ui.Canvas
	table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{Width: 300, Height: 120},
		[]TableColumn{{Width: 100}, {Width: 0}})
	table.Selection.Enabled = true
	table.Header([]string{"A", "B"})
	table.Row(ui.ID{}, []string{"one", "two"}, false)
	got := table.SelectedText(TableSelection{Anchor: 1000, Focus: -10})
	if got != "A\tB\none\ttwo" {
		t.Fatalf("reversed selection = %q", got)
	}
}

func TestTableCopySelectionClampsInsideUnicode(t *testing.T) {
	var canvas ui.Canvas
	table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{Width: 300, Height: 120},
		[]TableColumn{{Width: 100}, {Width: 0}})
	table.Selection.Enabled = true
	table.Header([]string{"名称", "值"})
	table.Row(ui.ID{}, []string{"中文", "测试"}, false)
	got := table.SelectedText(TableSelection{Anchor: 3, Focus: table.TextLen() - 1})
	if got != "称\t值\n中文\t测" {
		t.Fatalf("unicode selection = %q", got)
	}
}

func TestTableCopySelectionSerializesEmbeddedTSVDelimiters(t *testing.T) {
	var canvas ui.Canvas
	table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{Width: 400, Height: 200},
		[]TableColumn{{Width: 120}, {Width: 0}})
	table.Selection.Enabled = true
	table.Header([]string{"Variable", "Value"})
	rows := [][]string{
		{"TAB", "a\tb"},
		{"LINES", "line one\nline two\rreturn"},
		{"QUOTES", `"quoted""value"`},
		{"UNICODE", "中文"},
	}
	for _, row := range rows {
		table.Row(ui.ID{}, row, false)
	}

	got := table.SelectedText(TableSelection{Anchor: 0, Focus: table.TextLen()})
	reader := csv.NewReader(strings.NewReader(got))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("quoted TSV did not parse: %v\n%s", err, got)
	}
	want := append([][]string{{"Variable", "Value"}}, rows...)
	if !slices.EqualFunc(records, want, slices.Equal) {
		t.Fatalf("quoted TSV = %#v, want %#v\n%s", records, want, got)
	}
}

func TestTableCopySelectionMapsQuotedVisibleTextToSerializedOffset(t *testing.T) {
	theme := ui.DarkTheme()
	bounds := ui.Rect{X: 10, Y: 20, Width: 300, Height: 160}
	value := `a"b`
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, bounds, []TableColumn{{Width: 80}, {Width: 0}})
	table.Selection.Enabled = true
	table.Header([]string{"Key", "Value"})
	table.Row(ui.ID{}, []string{"QUOTE", value}, false)

	cell := table.text.cells[len(table.text.cells)-1]
	prefixWidth := canvas.TextWidthWeight(`a"`, theme.Metrics.FontSizeM, ui.WeightRegular)
	offset, ok := table.OffsetAt(ui.Point{
		X: cell.textX + prefixWidth,
		Y: cell.bounds.Y + 2,
	})
	want := cell.contentLo + len(`a"`) + 1 // the literal quote occupies two TSV bytes
	if !ok || offset != want {
		t.Fatalf("quoted pointer offset = %d, want %d (%v)", offset, want, ok)
	}
}

func TestTableCopySelectionIncludesQuotesWhenDraggingAWholeSpecialCell(t *testing.T) {
	theme := ui.DarkTheme()
	bounds := ui.Rect{X: 10, Y: 20, Width: 300, Height: 160}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, bounds, []TableColumn{{Width: 100}, {Width: 0}})
	table.Selection.Enabled = true
	table.Header([]string{"Key", "Value"})
	table.Row(ui.ID{}, []string{"QUOTE", "a\tb"}, false)

	first, second := table.text.cells[2], table.text.cells[3]
	start, ok := table.OffsetAt(ui.Point{X: first.textX, Y: first.bounds.Y + 2})
	if !ok {
		t.Fatal("start of row did not map")
	}
	end, ok := table.OffsetAt(ui.Point{X: second.bounds.Right() - 1, Y: second.bounds.Y + 2})
	if !ok {
		t.Fatal("end of row did not map")
	}
	got := table.SelectedText(TableSelection{Anchor: start, Focus: end})
	if got != "QUOTE\t\"a\tb\"" {
		t.Fatalf("dragged row = %q", got)
	}
}

func TestTableCopySelectionIncludesQuotesForRTLSpecialCells(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, ui.Rect{Width: 300, Height: 160},
		[]TableColumn{{Width: 100}, {Width: 0}})
	table.Selection.Enabled = true
	table.Header([]string{"Key", "Value"})
	table.Row(ui.ID{}, []string{"RTL", "שלום\tעולם"}, false)

	second := table.text.cells[3]
	start, ok := table.OffsetAt(ui.Point{X: second.textX + second.textWidth - 1, Y: second.bounds.Y + 2})
	if !ok {
		t.Fatal("RTL row start did not map")
	}
	end, ok := table.OffsetAt(ui.Point{X: second.textX + 1, Y: second.bounds.Y + 2})
	if !ok {
		t.Fatal("RTL row end did not map")
	}
	got := table.SelectedText(TableSelection{Anchor: start, Focus: end})
	if got != "\"שלום\tעולם\"" {
		t.Fatalf("dragged RTL row = %q", got)
	}
}

func TestTableCopySelectionColorsOnlySelectedText(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, ui.Rect{Width: 300, Height: 120},
		[]TableColumn{{Width: 100}, {Width: 0}})
	table.Selection = TableSelection{
		Enabled: true,
		Anchor:  len("Name\tValue\nkey\t") + 1,
		Focus:   len("Name\tValue\nkey\t") + 3,
	}
	table.Header([]string{"Name", "Value"})
	table.Row(ui.ID{}, []string{"key", "abcdef"}, false)

	normal, selected := false, false
	for _, text := range canvas.Texts() {
		if text.Value != "abcdef" {
			continue
		}
		normal = normal || text.Style.Color == theme.Foreground
		selected = selected || text.Style.Color == theme.SelectionForeground && text.HasClip
	}
	if !normal || !selected {
		t.Fatalf("cell text colors: normal=%v selected=%v texts=%+v", normal, selected, canvas.Texts())
	}
}

func TestTableCopySelectionIncludesSortableHeaderAndFooter(t *testing.T) {
	theme := ui.DarkTheme()
	bounds := ui.Rect{X: 15, Y: 25, Width: 340, Height: 180}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, bounds, []TableColumn{{Width: 100}, {Width: 0}})
	table.Selection.Enabled = true
	table.SortableHeader([]TableHeader{
		{ID: ui.Item(1), Label: "Variable", Sort: TableSortAscending},
		{Label: "Value"},
	})
	table.Row(ui.ID{}, []string{"LANG", "中文"}, false)
	table.Footer([]string{"Total", "1"})

	const want = "Variable\tValue\nLANG\t中文\nTotal\t1"
	if got := table.SelectedText(TableSelection{Anchor: 0, Focus: table.TextLen()}); got != want {
		t.Fatalf("sortable table text = %q, want %q", got, want)
	}
	headerOffset, ok := table.OffsetAt(ui.Point{
		X: bounds.X + theme.Metrics.GapItem + 1,
		Y: bounds.Y + theme.Metrics.ControlHeight/2,
	})
	if !ok || headerOffset < 0 || headerOffset >= len("Variable") {
		t.Fatalf("sortable header offset = %d, %v", headerOffset, ok)
	}
	footerY := bounds.Y + TableHeaderHeight(theme) + TableRowHeight(theme) + 2
	footerOffset, ok := table.OffsetAt(ui.Point{X: bounds.X + theme.Metrics.GapItem + 1, Y: footerY})
	if !ok || footerOffset < len("Variable\tValue\nLANG\t中文\n") {
		t.Fatalf("footer offset = %d, %v", footerOffset, ok)
	}
}

func TestTableCopySelectionUsesRTLPaintOrigin(t *testing.T) {
	theme := ui.DarkTheme()
	bounds := ui.Rect{X: 40, Y: 60, Width: 280, Height: 120}
	var canvas ui.Canvas
	table := BeginTable(&canvas, theme, bounds, []TableColumn{{Width: 120}, {Width: 0}})
	table.Selection.Enabled = true
	table.Header([]string{"مرحبا", "Value"})
	table.Row(ui.ID{}, []string{"שלום", "tail"}, false)

	var rtlPainted ui.Text
	for _, text := range canvas.Texts() {
		if text.Value == "مرحبا" {
			rtlPainted = text
			break
		}
	}
	if rtlPainted.Value == "" {
		t.Fatal("RTL header was not painted")
	}
	rtlCell := table.text.cells[0]
	if rtlCell.textX != rtlPainted.Bounds.X {
		t.Fatalf("RTL paint origin = %v, selection origin = %v", rtlPainted.Bounds.X, rtlCell.textX)
	}
	offset, ok := table.OffsetAt(ui.Point{
		X: rtlCell.textX + rtlCell.textWidth - 1,
		Y: bounds.Y + theme.Metrics.ControlHeight/2,
	})
	if !ok || offset > rtlCell.contentLo+1 {
		t.Fatalf("RTL right edge offset = %d, want near logical start %d (%v)",
			offset, rtlCell.contentLo, ok)
	}
	offset, ok = table.OffsetAt(ui.Point{
		X: rtlCell.textX + 1,
		Y: bounds.Y + theme.Metrics.ControlHeight/2,
	})
	if !ok || offset < rtlCell.contentHi-1 {
		t.Fatalf("RTL left edge offset = %d, want near logical end %d (%v)",
			offset, rtlCell.contentHi, ok)
	}
}

func TestTablePaintsCellsThatFitWithoutAllocating(t *testing.T) {
	columns := []TableColumn{{Width: 120}, {Width: 0}}
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{Width: 300, Height: 120}, columns)
		table.Header([]string{"Name", "Value"})
		table.Row(ui.Item(1), []string{"Region", "Europe"}, false)
	}
	if allocs := testing.AllocsPerRun(100, paint); allocs != 0 {
		t.Fatalf("table of fitting cells allocated %v times per paint", allocs)
	}
}

func TestTableSelectionWithAKeptStatePaintsWithoutAllocating(t *testing.T) {
	columns := []TableColumn{{Width: 120}, {Width: 0}}
	var canvas ui.Canvas
	state := &TableTextState{}
	paint := func() {
		canvas.Reset()
		table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{Width: 300, Height: 120}, columns)
		table.Selection = TableSelection{Enabled: true}
		table.UseSelectionState(state)
		table.Header([]string{"Name", "Value"})
		table.Row(ui.Item(1), []string{"Region", "Europe"}, false)
		table.Row(ui.Item(2), []string{"Coastline", "Barents Sea"}, false)
	}
	if allocs := testing.AllocsPerRun(100, paint); allocs != 0 {
		t.Fatalf("table with a kept selection state allocated %v times per paint", allocs)
	}
}

func tableTextValues(texts []ui.Text) []string {
	values := make([]string, 0, len(texts))
	for _, text := range texts {
		values = append(values, text.Value)
	}
	return values
}

func TestFitTableHeadingsWidensOnlyColumnsNarrowerThanTheirHeading(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	headings := []string{"", "Marker / hash", "Key type", "Comment", "Fingerprint"}
	columns := []TableColumn{{Width: 42}, {Width: 20}, {Width: 400}, {Width: 20, Inset: 10}, {Width: 0}}
	FitTableHeadings(&canvas, theme, columns, headings)
	pad := theme.Metrics.GapItem * 2
	fitted := canvas.TextWidthWeight("Marker / hash", theme.Metrics.FontSizeM, ui.WeightMedium)
	if columns[0].Width != 42 || columns[2].Width != 400 || columns[4].Width != 0 {
		t.Fatalf("columns that needed no change = %+v", columns)
	}
	if columns[1].Width < fitted+pad || columns[1].Width > fitted+pad+1 {
		t.Fatalf("heading column = %v, want %v for its text and padding", columns[1].Width, fitted+pad)
	}
	if columns[3].Width <= canvas.TextWidthWeight("Comment", theme.Metrics.FontSizeM, ui.WeightMedium)+pad+10-1 {
		t.Fatalf("inset column = %v does not hold the heading after its inset", columns[3].Width)
	}

	canvas.Reset()
	table := BeginTable(&canvas, theme, ui.Rect{Width: 800, Height: 80}, columns)
	table.Header(headings)
	for _, text := range canvas.Texts() {
		if strings.Contains(text.Value, "…") {
			t.Fatalf("fitted heading %q was ellipsized", text.Value)
		}
	}
}

func BenchmarkTablePaint(b *testing.B) {
	for _, bench := range []struct {
		name  string
		width float32
	}{{"fitting", 400}, {"ellipsized", 60}} {
		b.Run(bench.name, func(b *testing.B) {
			var canvas ui.Canvas
			columns := []TableColumn{{Width: bench.width}, {Width: bench.width}, {Width: 0}}
			rows := [][]string{
				{"known_hosts", "ssh-ed25519", "SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"},
				{"@cert-authority", "ecdsa-sha2-nistp256", "SHA256:uNiVztksCsDhcc0u9e8BujQXVUpKZIDTMczCvj3tD2s"},
			}
			b.ReportAllocs()
			for b.Loop() {
				canvas.Reset()
				table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{Width: 1000, Height: 200}, columns)
				table.Header([]string{"Marker / hash", "Key type", "Fingerprint"})
				for _, row := range rows {
					table.Row(ui.Item(1), row, false)
				}
			}
		})
	}
}

func BenchmarkTableSelectionPaint(b *testing.B) {
	for _, bench := range []struct {
		name  string
		state *TableTextState
	}{{"rebuilt", nil}, {"kept", &TableTextState{}}} {
		b.Run(bench.name, func(b *testing.B) {
			var canvas ui.Canvas
			columns := []TableColumn{{Width: 400}, {Width: 0}}
			rows := make([][]string, 30)
			for index := range rows {
				rows[index] = []string{"known_hosts", "SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"}
			}
			b.ReportAllocs()
			for b.Loop() {
				canvas.Reset()
				table := BeginTable(&canvas, ui.DarkTheme(), ui.Rect{Width: 1000, Height: 2000}, columns)
				table.Selection = TableSelection{Enabled: true}
				table.UseSelectionState(bench.state)
				table.Header([]string{"File", "Fingerprint"})
				for _, row := range rows {
					table.Row(ui.Item(1), row, false)
				}
			}
		})
	}
}
