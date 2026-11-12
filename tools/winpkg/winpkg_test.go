package winpkg

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func square(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / size), G: uint8(y * 255 / size), B: 90, A: 255})
		}
	}
	return img
}

// marked is a white square with a dark mark a quarter of its side, as an
// application draws its mark for a desktop that gives the tile a margin.
func marked(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for index := 0; index < len(img.Pix); index += 4 {
		img.Pix[index], img.Pix[index+1], img.Pix[index+2], img.Pix[index+3] = 255, 255, 255, 255
	}
	mark := size / 4
	for y := size/2 - mark/2; y < size/2+mark/2; y++ {
		for x := size/2 - mark/2; x < size/2+mark/2; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 16, G: 16, B: 16, A: 255})
		}
	}
	return img
}

type icoEntry struct {
	width, height, planes, bits int
	data                        []byte
}

// parseICO is a small independent reader for the test.
func parseICO(t *testing.T, data []byte) []icoEntry {
	t.Helper()
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:]) != 0 || binary.LittleEndian.Uint16(data[2:]) != 1 {
		t.Fatal("bad ICONDIR")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	var entries []icoEntry
	for index := 0; index < count; index++ {
		raw := data[6+16*index : 6+16*(index+1)]
		entry := icoEntry{width: int(raw[0]), height: int(raw[1]), planes: int(binary.LittleEndian.Uint16(raw[4:])), bits: int(binary.LittleEndian.Uint16(raw[6:]))}
		if entry.width == 0 {
			entry.width = 256
		}
		if entry.height == 0 {
			entry.height = 256
		}
		size, offset := int(binary.LittleEndian.Uint32(raw[8:])), int(binary.LittleEndian.Uint32(raw[12:]))
		entry.data = data[offset : offset+size]
		entries = append(entries, entry)
	}
	return entries
}

func TestEncodeICOHoldsEverySizeUpToTheSource(t *testing.T) {
	data, largest, err := EncodeICO(square(512))
	if err != nil {
		t.Fatal(err)
	}
	if largest != 256 {
		t.Errorf("largest = %d", largest)
	}
	entries := parseICO(t, data)
	var sizes []int
	for _, entry := range entries {
		sizes = append(sizes, entry.width)
		if entry.width != entry.height || entry.planes != 1 || entry.bits != 32 {
			t.Errorf("entry %+v", entry)
		}
		decoded, err := png.Decode(bytes.NewReader(entry.data))
		if err != nil {
			t.Fatalf("%d px: %v", entry.width, err)
		}
		if decoded.Bounds().Dx() != entry.width || decoded.Bounds().Dy() != entry.width {
			t.Errorf("%d px entry holds a %v image", entry.width, decoded.Bounds())
		}
	}
	want := iconSizes
	if len(sizes) != len(want) {
		t.Fatalf("sizes = %v", sizes)
	}
	for index := range want {
		if sizes[index] != want[index] {
			t.Fatalf("sizes = %v, want %v", sizes, want)
		}
	}
	if err := checkICO(data); err != nil {
		t.Error(err)
	}
}

func TestEncodeICOFillsTheFramesWindowsDrawsSmallWithTheMark(t *testing.T) {
	data, _, err := EncodeICO(marked(512))
	if err != nil {
		t.Fatal(err)
	}
	entries := parseICO(t, data)
	share := func(pixels int) float64 {
		for _, entry := range entries {
			if entry.width != pixels {
				continue
			}
			decoded, err := png.Decode(bytes.NewReader(entry.data))
			if err != nil {
				t.Fatalf("%d px: %v", pixels, err)
			}
			dark := 0
			for y := 0; y < pixels; y++ {
				for x := 0; x < pixels; x++ {
					r, _, _, a := decoded.At(x, y).RGBA()
					if a>>8 > 128 && r>>8 < 64 {
						dark++
					}
				}
			}
			return float64(dark) / float64(pixels*pixels)
		}
		t.Fatalf("no %d px entry", pixels)
		return 0
	}
	if small, big := share(32), share(256); small < 0.4 || big > 0.1 {
		t.Errorf("the 32 px frame is %.0f%% mark and the 256 px frame %.0f%%: the frames Windows draws small must carry the mark, the large ones the application's tile", small*100, big*100)
	}
	if medium := share(64); medium > 0.5 {
		t.Errorf("the 64 px frame is %.0f%% mark: only the frames Windows draws small crop the canvas margin", medium*100)
	}
}

func TestEncodeICODoesNotUpscale(t *testing.T) {
	data, largest, err := EncodeICO(square(130))
	if err != nil {
		t.Fatal(err)
	}
	entries := parseICO(t, data)
	if want := len(iconSizes) - 1; largest != 128 || entries[len(entries)-1].width != 128 || len(entries) != want {
		t.Errorf("largest %d, %d entries, want %d", largest, len(entries), want)
	}
	// A 256 entry is stored with width and height byte 0.
	big, _, _ := EncodeICO(square(256))
	last := 6 + 16*(len(iconSizes)-1)
	if big[6] != 16 || big[last] != 0 || big[last+1] != 0 {
		t.Errorf("directory bytes: %v", big[:6+16*len(iconSizes)])
	}
}

func TestEncodeICORejects(t *testing.T) {
	if _, _, err := EncodeICO(square(64)); err == nil || !strings.Contains(err.Error(), "at least 128 px") {
		t.Errorf("small icon: %v", err)
	}
	if _, _, err := EncodeICO(image.NewNRGBA(image.Rect(0, 0, 200, 150))); err == nil || !strings.Contains(err.Error(), "square") {
		t.Errorf("non-square icon: %v", err)
	}
}

func TestLoadIcon(t *testing.T) {
	dir := t.TempDir()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, square(300)); err != nil {
		t.Fatal(err)
	}
	pngPath := filepath.Join(dir, "icon.png")
	os.WriteFile(pngPath, buffer.Bytes(), 0o644)
	data, pixels, err := LoadIcon(pngPath)
	if err != nil || pixels != 256 || len(parseICO(t, data)) != len(iconSizes) {
		t.Fatalf("png: %v pixels %d", err, pixels)
	}

	icoPath := filepath.Join(dir, "icon.ico")
	os.WriteFile(icoPath, data, 0o644)
	again, pixels, err := LoadIcon(icoPath)
	if err != nil || pixels != 0 || !bytes.Equal(again, data) {
		t.Fatalf("ico: %v pixels %d", err, pixels)
	}

	bad := filepath.Join(dir, "bad.ico")
	os.WriteFile(bad, []byte("not an icon at all"), 0o644)
	if _, _, err := LoadIcon(bad); err == nil || !strings.Contains(err.Error(), "not an .ico file") {
		t.Errorf("bad ico: %v", err)
	}
	truncated := filepath.Join(dir, "truncated.ico")
	os.WriteFile(truncated, data[:len(data)-1], 0o644) // the last image is cut short
	if _, _, err := LoadIcon(truncated); err == nil || !strings.Contains(err.Error(), "outside the file") {
		t.Errorf("truncated ico: %v", err)
	}
	icns := filepath.Join(dir, "icon.icns")
	os.WriteFile(icns, []byte("icns...."), 0o644)
	if _, _, err := LoadIcon(icns); err == nil || !strings.Contains(err.Error(), ".png or .ico") {
		t.Errorf("icns: %v", err)
	}
	if _, _, err := LoadIcon(filepath.Join(dir, "missing.png")); err == nil {
		t.Error("missing icon accepted")
	}
}

func sample() Script {
	return Script{
		Name: "Fixture Notes", Version: "2.0.1", Publisher: "Example Ltd", URL: "https://example.com/notes",
		ID: "dev.example.notes", Executable: "fixture-notes.exe", Program: `/tmp/p/fixture-notes`,
		Icon: "/tmp/p/icon.ico", Output: "/tmp/out/setup.exe", Arch: "amd64",
	}
}

func generate(t *testing.T, script Script) string {
	t.Helper()
	data, err := script.Generate()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGenerateUserScope(t *testing.T) {
	got := generate(t, sample())
	for _, want := range []string{
		"Unicode true\n", "SetCompressor /SOLID lzma\n",
		`!define APP_NAME "Fixture Notes"`, `!define APP_ID "dev.example.notes"`, `!define APP_EXE "fixture-notes.exe"`,
		`!define APP_URL "https://example.com/notes"`, `!define APP_PUBLISHER "Example Ltd"`,
		"RequestExecutionLevel user\n", `InstallDir "$LOCALAPPDATA\Programs\${APP_NAME}"`,
		`OutFile "/tmp/out/setup.exe"`, `!define MUI_ICON "/tmp/p/icon.ico"`,
		`File "/oname=${APP_EXE}" "/tmp/p/fixture-notes"`, `File "/oname=${APP_ICON}" "/tmp/p/icon.ico"`,
		`!define APP_ICON "fixture-notes.ico"`,
		`CreateShortcut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_ICON}"`,
		`CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_ICON}"`,
		`WriteUninstaller "$INSTDIR\Uninstall.exe"`,
		`!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_ID}"`,
		`WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "${APP_NAME}"`,
		`"DisplayVersion" "${APP_VERSION}"`, `"Publisher" "${APP_PUBLISHER}"`, `"URLInfoAbout" "${APP_URL}"`,
		`"DisplayIcon" "$INSTDIR\${APP_ICON}"`, `"UninstallString" '"$INSTDIR\Uninstall.exe"'`,
		`"QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'`, `"InstallLocation" "$INSTDIR"`,
		`WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1`, `"NoRepair" 1`, `"EstimatedSize" $0`,
		`DeleteRegKey HKCU "${UNINSTALL_KEY}"`, `SetShellVarContext current`, `${IfNot} ${RunningX64}`,
		`VIProductVersion "2.0.1.0"`, `VIAddVersionKey /LANG=${LANG_ENGLISH} "FileVersion" "${APP_VERSION}"`,
		`VIAddVersionKey /LANG=${LANG_SIMPCHINESE} "ProductName" "${APP_NAME}"`,
		`!insertmacro MUI_LANGUAGE "English"`, `!insertmacro MUI_LANGUAGE "SimpChinese"`,
		"!define MUI_FINISHPAGE_SHOWREADME\n",
		`!define MUI_FINISHPAGE_SHOWREADME_TEXT "$(CREATE_DESKTOP_SHORTCUT)"`,
		`!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut`,
		`LangString CREATE_DESKTOP_SHORTCUT ${LANG_ENGLISH} "Create a desktop shortcut"`,
		`LangString CREATE_DESKTOP_SHORTCUT ${LANG_SIMPCHINESE} "创建桌面快捷方式"`,
		`${IfNot} ${Errors}` + "\n" + `    WriteRegDWORD HKCU "${UNINSTALL_KEY}" "DesktopShortcut" 1`,
		`!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"`, "/SD IDCANCEL IDRETRY retry",
		`RMDir "$INSTDIR"`, "  SetDateSave off\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the script lacks %q", want)
		}
	}
	for _, unwanted := range []string{"HKLM", "$PROGRAMFILES64", "RequestExecutionLevel admin", "IsWow64Process2", "RMDir /r"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("the user-scope script has %q", unwanted)
		}
	}
	// Uninstall removes exactly the files written.
	uninstall := got[strings.Index(got, `Section "Uninstall"`):]
	for _, want := range []string{
		`Delete "$INSTDIR\${APP_EXE}"`,
		`Delete "$INSTDIR\${APP_ICON}"`,
		`Delete "$INSTDIR\Uninstall.exe"`,
		`Delete "$SMPROGRAMS\${APP_NAME}.lnk"`,
		`ReadRegDWORD $0 HKCU "${UNINSTALL_KEY}" "DesktopShortcut"`,
		`${If} $0 == 1`,
		`Delete "$DESKTOP\${APP_NAME}.lnk"`,
	} {
		if !strings.Contains(uninstall, want) {
			t.Errorf("the uninstaller lacks %q", want)
		}
	}
}

// The program cannot start on Windows before 10: its Go runtime needs that
// version, so the installer must stop first. The manifest entry is part of the
// check: without it GetVersionEx reports 6.2 even on Windows 10.
func TestGenerateRefusesWindowsBefore10(t *testing.T) {
	got := generate(t, sample())
	for _, want := range []string{
		`!include "WinVer.nsh"`,
		"ManifestSupportedOS Win7 Win8 Win8.1 Win10\n",
		`LangString NEEDS_WINDOWS ${LANG_ENGLISH} "${APP_NAME} needs Windows 10 or Windows Server 2016 or later."`,
		`LangString NEEDS_WINDOWS ${LANG_SIMPCHINESE} "${APP_NAME} 需要 Windows 10 或 Windows Server 2016 或更高版本。"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the script lacks %q", want)
		}
	}
	init := got[strings.Index(got, `Function .onInit`):]
	init = init[:strings.Index(init, `FunctionEnd`)]
	guard := strings.Index(init, `${IfNot} ${AtLeastWin10}`)
	if guard < 0 {
		t.Fatal("the .onInit function does not check the Windows version")
	}
	if !strings.Contains(init[guard:], "Abort") {
		t.Error("the version check does not stop the install")
	}
	if !strings.Contains(init[guard:], "$(NEEDS_WINDOWS)") {
		t.Error("the version check does not say what is needed")
	}
}

func TestGenerateMachineScope(t *testing.T) {
	script := sample()
	script.Scope = ScopeMachine
	got := generate(t, script)
	for _, want := range []string{
		"RequestExecutionLevel admin\n", `InstallDir "$PROGRAMFILES64\${APP_NAME}"`, "SetShellVarContext all",
		`!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut`,
		`${IfNot} ${Errors}` + "\n" + `    WriteRegDWORD HKLM "${UNINSTALL_KEY}" "DesktopShortcut" 1`,
		`ReadRegDWORD $0 HKLM "${UNINSTALL_KEY}" "DesktopShortcut"`,
		`WriteRegStr HKLM "${UNINSTALL_KEY}" "DisplayName"`, `DeleteRegKey HKLM "${UNINSTALL_KEY}"`,
		`ReadRegStr $0 HKLM "${UNINSTALL_KEY}" "InstallLocation"`, "SetRegView 64",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the script lacks %q", want)
		}
	}
	for _, unwanted := range []string{"HKCU", "$LOCALAPPDATA", "MUI_FINISHPAGE_RUN"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("the machine-scope script has %q", unwanted)
		}
	}
}

func TestGenerateArm64AndNoIconNoURL(t *testing.T) {
	script := sample()
	script.Arch, script.Icon, script.URL, script.Publisher = "arm64", "", "", ""
	got := generate(t, script)
	for _, want := range []string{"IsWow64Process2", "43620", `!define APP_PUBLISHER "Fixture Notes"`,
		`CreateShortcut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"` + "\n",
		`CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"` + "\n", `"DisplayIcon" "$INSTDIR\${APP_EXE}"`} {
		if !strings.Contains(got, want) {
			t.Errorf("the script lacks %q", want)
		}
	}
	for _, unwanted := range []string{"RunningX64", "APP_URL", "MUI_ICON", "APP_ICON}\" \""} {
		if strings.Contains(got, unwanted) {
			t.Errorf("the script has %q", unwanted)
		}
	}
}

func TestGenerateEscapesEverything(t *testing.T) {
	script := sample()
	script.Name = `Cash $HOME's`
	script.Publisher = `Ex "Ltd" $(x)`
	script.URL = `https://example.com/?a=$1`
	script.Output = `/tmp/a"b/$out.exe`
	got := generate(t, script)
	for _, want := range []string{
		`!define APP_NAME "Cash $$HOME's"`, `!define APP_PUBLISHER "Ex $\"Ltd$\" $$(x)"`,
		`!define APP_URL "https://example.com/?a=$$1"`, `OutFile "/tmp/a$\"b/$$out.exe"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the script lacks %q\n%s", want, got)
		}
	}
	// The raw dollar sign never reaches the script outside an NSIS name.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "!define APP_") && strings.Contains(strings.ReplaceAll(line, "$$", ""), "$H") {
			t.Errorf("unescaped: %s", line)
		}
	}
}

func TestGenerateRejects(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Script)
		want   string
	}{
		{"newline in the publisher", func(s *Script) { s.Publisher = "a\nb" }, "publisher"},
		{"newline in the url", func(s *Script) { s.URL = "https://x/\r\n!system" }, "url"},
		{"newline in the output", func(s *Script) { s.Output = "a\nb" }, "output"},
		{"tab in the icon path", func(s *Script) { s.Icon = "a\tb" }, "icon"},
		{"name with a quote", func(s *Script) { s.Name = `A"B` }, "directory name"},
		{"name with a pipe", func(s *Script) { s.Name = "A|B" }, "directory name"},
		{"name ends in a dot", func(s *Script) { s.Name = "A." }, "directory name"},
		{"empty name", func(s *Script) { s.Name = "" }, "needs a name"},
		{"id with a slash", func(s *Script) { s.ID = "a/b" }, "identifier"},
		{"id with a dollar", func(s *Script) { s.ID = "a$b" }, "identifier"},
		{"version text", func(s *Script) { s.Version = "v1" }, "version"},
		{"version too large", func(s *Script) { s.Version = "70000.1" }, "65535"},
		{"executable without exe", func(s *Script) { s.Executable = "app" }, ".exe"},
		{"executable with a path", func(s *Script) { s.Executable = `bin\app.exe` }, ".exe"},
		{"scope", func(s *Script) { s.Scope = "all" }, "scope"},
		{"arch", func(s *Script) { s.Arch = "386" }, "architecture"},
		{"no program", func(s *Script) { s.Program = "" }, "required"},
	} {
		script := sample()
		test.change(&script)
		if _, err := script.Generate(); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %v", test.name, err)
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	first := generate(t, sample())
	for range 5 {
		if again := generate(t, sample()); again != first {
			t.Fatal("the script changed between runs")
		}
	}
	if strings.Contains(first, "\r") {
		t.Error("carriage return in the script")
	}
}

// A running program can be renamed but not deleted, so an update must move it
// aside before it ever asks the user, or a silent install fails with the first
// process that outlives the window.
func TestEnsureClosedMovesARunningProgramAsideBeforeAskingTheUser(t *testing.T) {
	got := generate(t, sample())
	// The function is written once, as a macro the installer and the uninstaller
	// each insert, so the text carries ${un} rather than either name.
	start := strings.Index(got, "Function ${un}EnsureClosed")
	if start < 0 {
		t.Fatalf("the script lacks EnsureClosed:\n%s", got)
	}
	body := got[start:]
	if end := strings.Index(body, "FunctionEnd"); end >= 0 {
		body = body[:end]
	}
	rename := strings.Index(body, `Rename "$INSTDIR\${APP_EXE}"`)
	message := strings.Index(body, "MessageBox")
	if rename < 0 || message < 0 {
		t.Fatalf("EnsureClosed lacks the rename or the question:\n%s", body)
	}
	if rename > message {
		t.Errorf("EnsureClosed asks the user before moving the program aside:\n%s", body)
	}
	if line := body[rename:]; !strings.HasPrefix(line[:strings.Index(line, "\n")], `Rename "$INSTDIR\${APP_EXE}" "$INSTDIR\${APP_EXE}.`) ||
		!strings.Contains(line[:strings.Index(line, "\n")], `.old"`) {
		t.Errorf("the program is not moved to a .old name:\n%s", body)
	}

	const leftovers = `Delete "$INSTDIR\${APP_EXE}.*.old"`
	install := got[strings.Index(got, `Section "${APP_NAME}" SecMain`):]
	install = install[:strings.Index(install, "SectionEnd")]
	if !strings.Contains(install, "Call EnsureClosed") || !strings.Contains(install, leftovers) {
		t.Errorf("the installer does not clear what an earlier update moved aside:\n%s", install)
	}
	uninstall := got[strings.Index(got, `Section "Uninstall"`):]
	if !strings.Contains(uninstall, leftovers) {
		t.Errorf("the uninstaller does not clear what an update moved aside:\n%s", uninstall)
	}
}

func TestFindReportsHowToInstall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := find("", nil)
	if !errors.Is(err, ErrNoNSIS) {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"brew install nsis", "apt install nsis", "nsis.sourceforge.io", "NSIS_HOME"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message lacks %q: %v", want, err)
		}
	}
}

func TestFindUsesNSISHomeAndTheLocations(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	name := "makensis"
	if os.PathSeparator == '\\' {
		name = "makensis.exe"
	}
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, "Bin"), 0o755)
	want := filepath.Join(home, "Bin", name)
	os.WriteFile(want, nil, 0o755)
	if got, err := find(home, nil); err != nil || got != want {
		t.Errorf("NSIS_HOME: %q, %v", got, err)
	}
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, name), nil, 0o755)
	if got, err := find("", []string{t.TempDir(), other}); err != nil || got != filepath.Join(other, name) {
		t.Errorf("locations: %q, %v", got, err)
	}
}

func TestWarningsAreExtracted(t *testing.T) {
	output := "Processed 1 file\nwarning: unknown variable \"x\" detected (a.nsi:3)\n   more\nwarning: second (a.nsi:4)\n"
	got := warnings(output)
	if len(got) != 2 || got[0] != `unknown variable "x" detected (a.nsi:3)` || got[1] != "second (a.nsi:4)" {
		t.Errorf("warnings = %q", got)
	}
}

// skipIfBroken skips when the machine's makensis cannot build Unicode installers
// at all (NSIS bug #1333 on macOS); nothing about the script is then tested.
func skipIfBroken(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, ErrMakensisCrash) {
		t.Skip(err)
	}
}

func TestBuildFailureCarriesMakensisOutput(t *testing.T) {
	makensis, err := Find()
	if err != nil {
		t.Skip("NSIS is not installed")
	}
	script := sample()
	script.Program = filepath.Join(t.TempDir(), "missing")
	script.Icon = ""
	script.Output = filepath.Join(t.TempDir(), "setup.exe")
	_, err = build(makensis, script)
	skipIfBroken(t, err)
	if err == nil || !strings.Contains(err.Error(), "makensis failed") {
		t.Errorf("err = %v", err)
	}
}

// buildable is sample() pointed at files that exist: NSIS opens the program and the
// icon while compiling, so a test that really builds an installer needs both on disk.
func buildable(t *testing.T, dir string) Script {
	t.Helper()
	program := filepath.Join(dir, "program")
	if err := os.WriteFile(program, append([]byte("MZ"), bytes.Repeat([]byte{7}, 4096)...), 0o644); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, square(256)); err != nil {
		t.Fatal(err)
	}
	pngPath := filepath.Join(dir, "icon.png")
	if err := os.WriteFile(pngPath, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	ico, _, err := LoadIcon(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(dir, "icon.ico")
	if err := os.WriteFile(icon, ico, 0o644); err != nil {
		t.Fatal(err)
	}
	script := sample()
	script.Program, script.Icon = program, icon
	return script
}

func TestBuildMakesAnInstaller(t *testing.T) {
	if _, err := Find(); err != nil {
		t.Skip("NSIS is not installed")
	}
	dir := t.TempDir()
	script := buildable(t, dir)
	for _, arch := range []string{"amd64", "arm64"} {
		for _, scope := range []string{ScopeUser, ScopeMachine} {
			script.Arch, script.Scope = arch, scope
			script.Output = filepath.Join(dir, arch+"-"+scope+"-setup.exe")
			result, err := Build(script)
			skipIfBroken(t, err)
			if err != nil {
				t.Fatalf("%s %s: %v", arch, scope, err)
			}
			if len(result.Warnings) != 0 {
				t.Errorf("%s %s: warnings %q", arch, scope, result.Warnings)
			}
			head, err := os.ReadFile(script.Output)
			if err != nil || len(head) < 2 || string(head[:2]) != "MZ" {
				t.Errorf("%s %s: not a PE file (%v)", arch, scope, err)
			}
		}
	}
}

func TestBuildIsReproducible(t *testing.T) {
	if _, err := Find(); err != nil {
		t.Skip("NSIS is not installed")
	}
	dir := t.TempDir()
	program := filepath.Join(dir, "program")
	os.WriteFile(program, append([]byte("MZ"), bytes.Repeat([]byte{9}, 2048)...), 0o644)
	make := func(name string) []byte {
		script := sample()
		script.Program, script.Icon = program, ""
		script.Output = filepath.Join(dir, name)
		_, err := Build(script)
		skipIfBroken(t, err)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(script.Output)
		return data
	}
	first := make("a.exe")
	later := zipTime.AddDate(40, 0, 0)
	os.Chtimes(program, later, later) // the program's modification time must not matter
	if !bytes.Equal(first, make("b.exe")) {
		t.Error("the installer depends on the program's modification time")
	}
}

func TestZipProgramIsReproducible(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "p")
	os.WriteFile(program, []byte("MZ program bytes"), 0o644)
	write := func(name string) []byte {
		path := filepath.Join(dir, name)
		if err := ZipProgram(program, "notes.exe", path); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(path)
		return data
	}
	first := write("a.zip")
	future := zipTime.AddDate(40, 0, 0)
	os.Chtimes(program, future, future)
	if !bytes.Equal(first, write("b.zip")) {
		t.Error("the archive depends on the file's timestamp")
	}
	reader, err := zip.OpenReader(filepath.Join(dir, "a.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != 1 || reader.File[0].Name != "notes.exe" || !reader.File[0].Modified.Equal(zipTime) {
		t.Fatalf("entries: %+v", reader.File)
	}
	entry, _ := reader.File[0].Open()
	if body, _ := io.ReadAll(entry); string(body) != "MZ program bytes" {
		t.Errorf("body = %q", body)
	}
	if err := ZipProgram(filepath.Join(dir, "missing"), "x.exe", filepath.Join(dir, "c.zip")); err == nil {
		t.Error("missing program accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "c.zip")); err == nil {
		t.Error("a failed run left an archive")
	}
}

func TestGenerateInstallsAndRemovesExtraFiles(t *testing.T) {
	script := sample()
	script.Files = []ExtraFile{
		{Source: "/tmp/p/conpty/x64/conpty.dll", Destination: "conpty/x64/conpty.dll"},
		{Source: "/tmp/p/conpty/x64/x64/OpenConsole.exe", Destination: "conpty/x64/x64/OpenConsole.exe"},
	}
	got := generate(t, script)
	for _, want := range []string{
		`  SetOutPath "$INSTDIR\conpty\x64"`,
		`  File "/oname=conpty.dll" "/tmp/p/conpty/x64/conpty.dll"`,
		`  SetOutPath "$INSTDIR\conpty\x64\x64"`,
		`  File "/oname=OpenConsole.exe" "/tmp/p/conpty/x64/x64/OpenConsole.exe"`,
		`  SetOutPath "$INSTDIR"`,
		`  Delete "$INSTDIR\conpty\x64\conpty.dll"`,
		`  Delete "$INSTDIR\conpty\x64\x64\OpenConsole.exe"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the script lacks %q", want)
		}
	}
	// The directory holding the files is removed before its parent, and both
	// before the installation directory.
	deep := strings.Index(got, `RMDir "$INSTDIR\conpty\x64\x64"`)
	shallow := strings.Index(got, `RMDir "$INSTDIR\conpty\x64"`)
	root := strings.LastIndex(got, `RMDir "$INSTDIR"`)
	if deep < 0 || shallow < 0 || root < 0 || !(deep < shallow && shallow < root) {
		t.Errorf("uninstall order: deep=%d shallow=%d root=%d", deep, shallow, root)
	}
}

func TestExtraFileValidation(t *testing.T) {
	for _, destination := range []string{
		"", "/conpty/x64.dll", "conpty/../x64.dll", `conpty\x64.dll`, "conpty//x64.dll",
		"conpty/.", "conpty/ x64.dll", "conpty/x64.dll.", "conpty/$x64.dll",
	} {
		files := []ExtraFile{{Source: "/tmp/p/x", Destination: destination}}
		if _, err := checkedExtraFiles(files); err == nil {
			t.Errorf("destination %q was accepted", destination)
		}
	}
	good := []ExtraFile{
		{Source: "/tmp/p/b", Destination: "conpty/arm64/OpenConsole.exe"},
		{Source: "/tmp/p/a", Destination: "conpty/LICENSE.txt"},
		{Source: "/tmp/p/c", Destination: "conpty/arm64/OpenConsole.exe"},
	}
	if _, err := checkedExtraFiles(good); err == nil {
		t.Error("a duplicate destination was accepted")
	}
	if err := CheckExtraFiles([]ExtraFile{{Source: filepath.Join(t.TempDir(), "missing"), Destination: "conpty/x"}}); err == nil {
		t.Error("a missing source was accepted")
	}
}

func TestZipProgramIncludesExtraFilesInDestinationOrder(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "p")
	os.WriteFile(program, []byte("MZ program bytes"), 0o644)
	written := map[string]string{
		"beta":  "OpenConsole bytes",
		"alpha": "conpty.dll bytes",
	}
	for name, body := range written {
		os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
	}
	archive := filepath.Join(dir, "a.zip")
	err := ZipProgram(program, "notes.exe", archive,
		ExtraFile{Source: filepath.Join(dir, "beta"), Destination: "conpty/x64/OpenConsole.exe"},
		ExtraFile{Source: filepath.Join(dir, "alpha"), Destination: "conpty/x64/conpty.dll"},
	)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var names []string
	for _, entry := range reader.File {
		names = append(names, entry.Name)
	}
	want := []string{"notes.exe", "conpty/x64/OpenConsole.exe", "conpty/x64/conpty.dll"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("entries = %v", names)
	}
	for index, name := range []string{"conpty/x64/OpenConsole.exe", "conpty/x64/conpty.dll"} {
		entry, err := reader.File[index+1].Open()
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(entry)
		if string(body) != written[[]string{"beta", "alpha"}[index]] {
			t.Errorf("%s body = %q", name, body)
		}
	}
	if err := ZipProgram(program, "notes.exe", filepath.Join(dir, "b.zip"),
		ExtraFile{Source: program, Destination: "notes.exe"}); err == nil {
		t.Error("an extra file colliding with the program was accepted")
	}
}

func TestGenerateRegistersLinkSchemes(t *testing.T) {
	script := sample()
	script.URLSchemes = []string{"ssh"}
	got := generate(t, script)
	for _, want := range []string{
		`WriteRegStr HKCU "Software\Classes\ssh" "URL Protocol" ""`,
		`WriteRegStr HKCU "Software\Classes\ssh\shell\open\command" "" '"$INSTDIR\${APP_EXE}" "%1"'`,
		`DeleteRegKey HKCU "Software\Classes\ssh"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("script lacks %q", want)
		}
	}
	script.URLSchemes = []string{"bad scheme"}
	script.Output, script.Program = "/tmp/out/setup.exe", "/tmp/p/x"
	if _, err := script.Generate(); err == nil {
		t.Error("an invalid scheme was accepted")
	}
}

func TestGenerateRegistersFileTypes(t *testing.T) {
	script := sample()
	script.FileTypes = []FileType{{Name: "Notes document", Extensions: []string{"note", "nt"}}}
	got := generate(t, script)
	progID := script.ID + ".note"
	for _, want := range []string{
		`WriteRegStr HKCU "Software\Classes\` + progID + `" "" "Notes document"`,
		`WriteRegStr HKCU "Software\Classes\` + progID + `" "FriendlyTypeName" "Notes document"`,
		`WriteRegStr HKCU "Software\Classes\` + progID + `\shell\open\command" "" '"$INSTDIR\${APP_EXE}" "%1"'`,
		`WriteRegStr HKCU "Software\Classes\Applications\${APP_EXE}" "FriendlyAppName" "${APP_NAME}"`,
		`WriteRegStr HKCU "Software\Classes\Applications\${APP_EXE}" "ApplicationCompany" "${APP_PUBLISHER}"`,
		`WriteRegStr HKCU "Software\Classes\Applications\${APP_EXE}\SupportedTypes" ".note" ""`,
		`WriteRegStr HKCU "Software\RegisteredApplications" "${APP_ID}" "${APP_CAPABILITIES}"`,
		`WriteRegStr HKCU "${APP_CAPABILITIES}" "ApplicationIcon" '"$INSTDIR\${APP_EXE}",0'`,
		`WriteRegStr HKCU "${APP_CAPABILITIES}\FileAssociations" ".note" "` + progID + `"`,
		`WriteRegStr HKCU "Software\Classes\.note\OpenWithProgids" "` + progID + `" ""`,
		`WriteRegStr HKCU "Software\Classes\.nt\OpenWithProgids" "` + progID + `" ""`,
		`DeleteRegValue HKCU "Software\Classes\.nt\OpenWithProgids" "` + progID + `"`,
		`DeleteRegKey HKCU "Software\Classes\` + progID + `"`,
		`DeleteRegKey HKCU "Software\Classes\Applications\${APP_EXE}"`,
		`DeleteRegValue HKCU "Software\RegisteredApplications" "${APP_ID}"`,
		`DeleteRegKey HKCU "${APP_CAPABILITIES}"`,
		`SHChangeNotify`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("script lacks %q", want)
		}
	}
	for _, bad := range []FileType{{Name: "x"}, {Name: "x", Extensions: []string{".md"}}, {Name: "a\nb", Extensions: []string{"md"}}} {
		script.FileTypes = []FileType{bad}
		script.Output, script.Program = "/tmp/out/setup.exe", "/tmp/p/x"
		if _, err := script.Generate(); err == nil {
			t.Errorf("%+v was accepted", bad)
		}
	}
}

func TestBuildWithFileTypesAndLinks(t *testing.T) {
	if _, err := Find(); err != nil {
		t.Skip("NSIS is not installed")
	}
	dir := t.TempDir()
	script := buildable(t, dir)
	script.Arch, script.Output = "amd64", filepath.Join(dir, "setup.exe")
	script.URLSchemes = []string{"ssh"}
	script.FileTypes = []FileType{{Name: "Notes document", Extensions: []string{"note", "nt"}}}
	for _, scope := range []string{ScopeUser, ScopeMachine} {
		script.Scope = scope
		result, err := Build(script)
		skipIfBroken(t, err)
		if err != nil {
			t.Fatalf("%s: %v", scope, err)
		}
		if len(result.Warnings) != 0 {
			t.Errorf("%s: warnings %q", scope, result.Warnings)
		}
	}
}
