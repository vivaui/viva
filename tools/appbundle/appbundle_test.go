package appbundle

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func sampleInfo() Info {
	return Info{
		Name: "Viva Gallery", ID: "dev.viva.gallery", Version: "1.2.0", Executable: "viva-gallery",
		MinSystem: "12.0", Category: "public.app-category.developer-tools", Copyright: "© 2026 Viva & <Co>",
	}
}

func TestPlistIsDeterministicAndEscaped(t *testing.T) {
	info := sampleInfo()
	info.IconFile = "AppIcon.icns"
	info.Extra = []KeyValue{{"NSMicrophoneUsageDescription", "Records \"audio\" & <more>"}, {"NSCameraUsageDescription", "Shows video."}}
	first, err := info.Plist()
	if err != nil {
		t.Fatal(err)
	}
	reversed := info
	reversed.Extra = []KeyValue{info.Extra[1], info.Extra[0]}
	second, _ := reversed.Plist()
	if !bytes.Equal(first, second) {
		t.Fatal("the order of extra entries changed the output")
	}
	text := string(first)
	for _, want := range []string{
		"<key>CFBundleIdentifier</key>\n\t<string>dev.viva.gallery</string>",
		"<key>CFBundleExecutable</key>\n\t<string>viva-gallery</string>",
		"<key>CFBundleIconFile</key>\n\t<string>AppIcon.icns</string>",
		"<key>CFBundleShortVersionString</key>\n\t<string>1.2.0</string>",
		"<key>LSMinimumSystemVersion</key>\n\t<string>12.0</string>",
		"<key>NSHighResolutionCapable</key>\n\t<true/>",
		"<key>NSPrincipalClass</key>\n\t<string>NSApplication</string>",
		"© 2026 Viva &amp; &lt;Co&gt;",
		"Records &#34;audio&#34; &amp; &lt;more&gt;",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Info.plist is missing %q", want)
		}
	}
	if strings.Index(text, "NSCameraUsageDescription") > strings.Index(text, "NSMicrophoneUsageDescription") {
		t.Error("extra entries are not sorted by key")
	}
}

func TestPlistOmitsWhatIsNotSet(t *testing.T) {
	info := sampleInfo()
	info.Category, info.Copyright, info.MinSystem = "", "", ""
	data, err := info.Plist()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"CFBundleIconFile", "LSApplicationCategoryType", "NSHumanReadableCopyright", "LSMinimumSystemVersion"} {
		if strings.Contains(string(data), key) {
			t.Errorf("%s was written although it is not set", key)
		}
	}
}

func TestPlistRejectsMissingFieldsAndClashingExtras(t *testing.T) {
	for _, mutate := range []func(*Info){
		func(i *Info) { i.Name = "" }, func(i *Info) { i.ID = "" }, func(i *Info) { i.Version = "" }, func(i *Info) { i.Executable = "" },
	} {
		info := sampleInfo()
		mutate(&info)
		if _, err := info.Plist(); err == nil {
			t.Errorf("accepted %+v", info)
		}
	}
	for _, key := range []string{"CFBundleIdentifier", "LSMinimumSystemVersion", "NSPrincipalClass"} {
		info := sampleInfo()
		info.Extra = []KeyValue{{key, "x"}}
		if _, err := info.Plist(); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("an extra %s was accepted: %v", key, err)
		}
	}
	info := sampleInfo()
	info.Extra = []KeyValue{{"NSX", "a"}, {"NSX", "b"}}
	if _, err := info.Plist(); err == nil {
		t.Error("a repeated extra key was accepted")
	}
}

func square(size int, paint func(x, y int) color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, paint(x, y))
		}
	}
	return img
}

// readICNS splits an .icns file into its entries.
func readICNS(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	if string(data[:4]) != "icns" || int(binary.BigEndian.Uint32(data[4:8])) != len(data) {
		t.Fatalf("bad icns header: % x, length %d", data[:8], len(data))
	}
	entries := map[string][]byte{}
	for offset := 8; offset < len(data); {
		kind, length := string(data[offset:offset+4]), int(binary.BigEndian.Uint32(data[offset+4:offset+8]))
		if length < 8 || offset+length > len(data) {
			t.Fatalf("entry %s at %d has length %d", kind, offset, length)
		}
		entries[kind] = data[offset+8 : offset+length]
		offset += length
	}
	return entries
}

func TestEncodeICNSHoldsEverySizeAsPNG(t *testing.T) {
	data, largest, err := EncodeICNS(square(1024, func(x, y int) color.NRGBA { return color.NRGBA{R: 200, G: 30, B: 60, A: 255} }))
	if err != nil || largest != 1024 {
		t.Fatalf("largest %d, err %v", largest, err)
	}
	entries := readICNS(t, data)
	want := map[string]int{"ic11": 32, "ic12": 64, "ic07": 128, "ic13": 256, "ic08": 256, "ic14": 512, "ic09": 512, "ic10": 1024}
	if len(entries) != len(want) {
		t.Fatalf("%d entries, want %d", len(entries), len(want))
	}
	// IconServices draws a PNG in these two entries as noise; see icnsKinds.
	for _, kind := range []string{"icp4", "icp5"} {
		if _, present := entries[kind]; present {
			t.Errorf("%s holds a PNG, which macOS cannot read there", kind)
		}
	}
	for kind, pixels := range want {
		decoded, err := png.Decode(bytes.NewReader(entries[kind]))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if bounds := decoded.Bounds(); bounds.Dx() != pixels || bounds.Dy() != pixels {
			t.Errorf("%s is %dx%d, want %d", kind, bounds.Dx(), bounds.Dy(), pixels)
		}
		if got := color.NRGBAModel.Convert(decoded.At(pixels/2, pixels/2)).(color.NRGBA); got != (color.NRGBA{R: 200, G: 30, B: 60, A: 255}) {
			t.Errorf("%s: a solid color came back as %+v", kind, got)
		}
	}
}

func TestEncodeICNSDoesNotUpscale(t *testing.T) {
	data, largest, err := EncodeICNS(square(256, func(x, y int) color.NRGBA { return color.NRGBA{A: 255} }))
	if err != nil || largest != 256 {
		t.Fatalf("largest %d, err %v", largest, err)
	}
	entries := readICNS(t, data)
	for _, kind := range []string{"ic14", "ic09", "ic10"} {
		if _, present := entries[kind]; present {
			t.Errorf("%s was made from a 256 px source", kind)
		}
	}
	for _, kind := range []string{"ic11", "ic07", "ic13", "ic08"} {
		if _, present := entries[kind]; !present {
			t.Errorf("%s is missing", kind)
		}
	}
}

func TestEncodeICNSRejectsUnusableSources(t *testing.T) {
	if _, _, err := EncodeICNS(image.NewNRGBA(image.Rect(0, 0, 512, 256))); err == nil || !strings.Contains(err.Error(), "square") {
		t.Errorf("a non-square icon: %v", err)
	}
	if _, _, err := EncodeICNS(image.NewNRGBA(image.Rect(0, 0, 64, 64))); err == nil || !strings.Contains(err.Error(), "128") {
		t.Errorf("a tiny icon: %v", err)
	}
}

func TestDownscaleAveragesByAreaOnPremultipliedColor(t *testing.T) {
	// A checkerboard of opaque black and white halves to mid gray.
	checker := square(256, func(x, y int) color.NRGBA {
		if (x+y)%2 == 0 {
			return color.NRGBA{A: 255}
		}
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	})
	got := Downscale(checker, 128).NRGBAAt(10, 10)
	if got.A != 255 || got.R < 126 || got.R > 129 {
		t.Errorf("black and white averaged to %+v, want mid gray", got)
	}

	// Half of the area is transparent: the color of the opaque half survives
	// (no darkening from the transparent black) and alpha is the coverage.
	half := square(256, func(x, y int) color.NRGBA {
		if x%2 == 0 {
			return color.NRGBA{R: 255, G: 128, B: 0, A: 255}
		}
		return color.NRGBA{}
	})
	got = Downscale(half, 128).NRGBAAt(10, 10)
	if got.R < 253 || got.G < 126 || got.G > 130 || got.B > 2 || got.A < 126 || got.A > 129 {
		t.Errorf("half-transparent orange became %+v", got)
	}

	// A fully transparent area stays transparent.
	if got := Downscale(square(256, func(x, y int) color.NRGBA { return color.NRGBA{} }), 128).NRGBAAt(5, 5); got.A != 0 {
		t.Errorf("transparent became %+v", got)
	}

	// A non-integer ratio still reaches every destination pixel.
	oddly := Downscale(square(300, func(x, y int) color.NRGBA { return color.NRGBA{R: 10, G: 20, B: 30, A: 255} }), 128)
	if got := oddly.NRGBAAt(127, 127); got != (color.NRGBA{R: 10, G: 20, B: 30, A: 255}) {
		t.Errorf("the last pixel of a 300->128 scale is %+v", got)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeProgram(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "program")
	if err := os.WriteFile(path, []byte("\xcf\xfa\xed\xfe not really a program"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAssembleBuildsTheBundleLayout(t *testing.T) {
	dir := t.TempDir()
	icon := filepath.Join(dir, "icon.png")
	writePNG(t, icon, square(512, func(x, y int) color.NRGBA { return color.NRGBA{R: 1, G: 2, B: 3, A: 255} }))
	dest := filepath.Join(dir, "out", "Viva Gallery.app")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Assemble(Options{Dest: dest, Info: sampleInfo(), Executable: fakeProgram(t, dir), Icon: icon})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != dest || result.IconPixels != 512 || result.ExecutableBytes == 0 || result.BundleBytes <= result.ExecutableBytes {
		t.Fatalf("result = %+v", result)
	}
	program, err := os.Stat(filepath.Join(dest, "Contents", "MacOS", "viva-gallery"))
	if err != nil || !program.Mode().IsRegular() || runtime.GOOS != "windows" && program.Mode()&0o111 == 0 {
		t.Fatalf("the program is missing or not executable: %v %v", program, err)
	}
	plist, err := os.ReadFile(filepath.Join(dest, "Contents", "Info.plist"))
	if err != nil || !strings.Contains(string(plist), "AppIcon.icns") {
		t.Fatalf("Info.plist: %v\n%s", err, plist)
	}
	icns, err := os.ReadFile(filepath.Join(dest, "Contents", "Resources", IconFileName))
	if err != nil {
		t.Fatal(err)
	}
	readICNS(t, icns)
	if _, err := os.Stat(dest + ".viva-tmp"); !os.IsNotExist(err) {
		t.Error("the staging directory was left behind")
	}
}

func TestAssembleWithoutAnIcon(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "Plain.app")
	result, err := Assemble(Options{Dest: dest, Info: sampleInfo(), Executable: fakeProgram(t, dir)})
	if err != nil || result.IconPixels != 0 {
		t.Fatalf("result %+v, err %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "Contents", "Resources")); !os.IsNotExist(err) {
		t.Error("an empty Resources directory was created")
	}
	if plist, _ := os.ReadFile(filepath.Join(dest, "Contents", "Info.plist")); strings.Contains(string(plist), "CFBundleIconFile") {
		t.Error("Info.plist names an icon that is not there")
	}
}

func TestAssembleReplacesAPreviousBundleOnly(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "App.app")
	program := fakeProgram(t, dir)
	if _, err := Assemble(Options{Dest: dest, Info: sampleInfo(), Executable: program}); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dest, "Contents", "stale")
	if err := os.WriteFile(stale, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Assemble(Options{Dest: dest, Info: sampleInfo(), Executable: program}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a file of the previous bundle survived the rebuild")
	}

	// A directory that merely has the right name is not a bundle.
	notBundle := filepath.Join(dir, "Documents.app")
	if err := os.MkdirAll(notBundle, 0o755); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(notBundle, "precious.txt")
	if err := os.WriteFile(precious, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Assemble(Options{Dest: notBundle, Info: sampleInfo(), Executable: program}); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(precious); err != nil {
		t.Error("a directory that is not a bundle was deleted")
	}

	// Neither is a plain file, nor a destination without the .app suffix.
	file := filepath.Join(dir, "File.app")
	os.WriteFile(file, []byte("x"), 0o644)
	if _, err := Assemble(Options{Dest: file, Info: sampleInfo(), Executable: program}); err == nil {
		t.Error("a file was replaced")
	}
	if _, err := Assemble(Options{Dest: filepath.Join(dir, "App"), Info: sampleInfo(), Executable: program}); err == nil {
		t.Error("a destination without .app was accepted")
	}
}

func TestAssembleFailureLeavesThePreviousBundle(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "App.app")
	program := fakeProgram(t, dir)
	if _, err := Assemble(Options{Dest: dest, Info: sampleInfo(), Executable: program}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dest, "Contents", "Info.plist"))
	broken := sampleInfo()
	broken.Extra = []KeyValue{{"CFBundleIdentifier", "x"}}
	if _, err := Assemble(Options{Dest: dest, Info: broken, Executable: program}); err == nil {
		t.Fatal("a bad Info was accepted")
	}
	if _, err := Assemble(Options{Dest: dest, Info: sampleInfo(), Executable: program, Icon: filepath.Join(dir, "missing.png")}); err == nil {
		t.Fatal("a missing icon was accepted")
	}
	if _, err := Assemble(Options{Dest: dest, Info: sampleInfo(), Executable: filepath.Join(dir, "missing")}); err == nil {
		t.Fatal("a missing program was accepted")
	}
	after, err := os.ReadFile(filepath.Join(dest, "Contents", "Info.plist"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("a failed build damaged the previous bundle")
	}
}

func TestLoadIcon(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "a.icns")
	os.WriteFile(good, []byte("icns\x00\x00\x00\x08"), 0o644)
	if data, _, err := LoadIcon(good); err != nil || string(data[:4]) != "icns" {
		t.Errorf("an .icns was refused: %v", err)
	}
	bad := filepath.Join(dir, "b.icns")
	os.WriteFile(bad, []byte("not an icon"), 0o644)
	if _, _, err := LoadIcon(bad); err == nil {
		t.Error("a bogus .icns was accepted")
	}
	if _, _, err := LoadIcon(filepath.Join(dir, "c.jpg")); err == nil {
		t.Error("a .jpg was accepted")
	}
	broken := filepath.Join(dir, "d.png")
	os.WriteFile(broken, []byte("not a png"), 0o644)
	if _, _, err := LoadIcon(broken); err == nil {
		t.Error("a bogus .png was accepted")
	}
	small := filepath.Join(dir, "e.png")
	writePNG(t, small, square(64, func(x, y int) color.NRGBA { return color.NRGBA{A: 255} }))
	if _, _, err := LoadIcon(small); err == nil || !strings.Contains(err.Error(), "e.png") {
		t.Errorf("a small png: %v", err)
	}
}

func TestZipKeepsTheLayoutModesAndIsReproducible(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "Viva Gallery.app")
	if _, err := Assemble(Options{Dest: dest, Info: sampleInfo(), Executable: fakeProgram(t, dir)}); err != nil {
		t.Fatal(err)
	}
	first, second := filepath.Join(dir, "a.zip"), filepath.Join(dir, "b.zip")
	if err := Zip(dest, first); err != nil {
		t.Fatal(err)
	}
	if err := Zip(dest, second); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(first)
	b, _ := os.ReadFile(second)
	if !bytes.Equal(a, b) {
		t.Error("zipping the same bundle twice gave different bytes")
	}

	reader, err := zip.OpenReader(first)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var names []string
	for _, file := range reader.File {
		names = append(names, file.Name)
		switch file.Name {
		case "Viva Gallery.app/Contents/MacOS/viva-gallery":
			if file.Mode()&0o111 == 0 {
				t.Error("the program lost its executable bit")
			}
			handle, _ := file.Open()
			content, _ := io.ReadAll(handle)
			handle.Close()
			if !strings.Contains(string(content), "not really a program") {
				t.Error("the program's bytes changed")
			}
		case "Viva Gallery.app/Contents/Info.plist":
			if file.Mode()&0o111 != 0 {
				t.Error("Info.plist became executable")
			}
		}
		if !file.Modified.Equal(zipTime) {
			t.Errorf("%s has timestamp %v", file.Name, file.Modified)
		}
	}
	want := []string{
		"Viva Gallery.app/", "Viva Gallery.app/Contents/", "Viva Gallery.app/Contents/Info.plist",
		"Viva Gallery.app/Contents/MacOS/", "Viva Gallery.app/Contents/MacOS/viva-gallery",
	}
	if !slices.Equal(names, want) {
		t.Errorf("entries = %q\nwant      %q", names, want)
	}
	if err := Zip(filepath.Join(dir, "Nothing.app"), filepath.Join(dir, "c.zip")); err == nil {
		t.Error("zipped something that is not a bundle")
	}
}

// The system tools are the authority on what a bundle's files mean.
func TestSystemToolsAcceptTheBundle(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("plutil and iconutil are macOS tools")
	}
	dir := t.TempDir()
	icon := filepath.Join(dir, "icon.png")
	writePNG(t, icon, square(1024, func(x, y int) color.NRGBA { return color.NRGBA{R: uint8(x / 4), G: uint8(y / 4), B: 128, A: 255} }))
	dest := filepath.Join(dir, "Viva Gallery.app")
	info := sampleInfo()
	info.Extra = []KeyValue{{"NSMicrophoneUsageDescription", "Records \"audio\" & <more>"}}
	if _, err := Assemble(Options{Dest: dest, Info: info, Executable: fakeProgram(t, dir), Icon: icon}); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("plutil", "-lint", filepath.Join(dest, "Contents", "Info.plist")).CombinedOutput(); err != nil {
		t.Fatalf("plutil -lint: %v\n%s", err, output)
	}
	iconset := filepath.Join(dir, "out.iconset")
	if output, err := exec.Command("iconutil", "--convert", "iconset", "--output", iconset, filepath.Join(dest, "Contents", "Resources", IconFileName)).CombinedOutput(); err != nil {
		t.Fatalf("iconutil rejected the icon: %v\n%s", err, output)
	}
	entries, _ := os.ReadDir(iconset)
	// The plain 16 and 32 px images are not stored (see icnsKinds); the @2x of 16
	// and 32 are, and make the rest.
	if len(entries) < 8 {
		t.Errorf("iconutil found %d images, want 8", len(entries))
	}
}

func TestFormatPackedVersion(t *testing.T) {
	for packed, want := range map[uint32]string{
		12 << 16: "12.0", 15<<16 | 5<<8: "15.5", 13<<16 | 3<<8 | 1: "13.3.1", 11 << 16: "11.0",
	} {
		if got := formatPackedVersion(packed); got != want {
			t.Errorf("%#x = %q, want %q", packed, got, want)
		}
	}
}

func TestRequiredMacOSReadsTheLinkersDeploymentTarget(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("needs a Mach-O program")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	os.WriteFile(source, []byte("package main\nfunc main(){}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module probe\ngo 1.26\n"), 0o644)
	binary := filepath.Join(dir, "probe")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = dir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}
	got, err := RequiredMacOS(binary)
	if err != nil {
		t.Fatal(err)
	}
	// Whatever Go chose, it is a plausible macOS version and the same one the
	// system's own tool reports.
	if !strings.Contains(got, ".") {
		t.Fatalf("version = %q", got)
	}
	if output, err := exec.Command("otool", "-l", binary).Output(); err == nil {
		if !strings.Contains(string(output), "minos "+got+"\n") && !strings.Contains(string(output), "version "+got+"\n") {
			t.Errorf("otool disagrees with %q:\n%s", got, grepLines(string(output), "minos", "version"))
		}
	}

	if _, err := RequiredMacOS(source); err == nil {
		t.Error("a Go source file was read as a Mach-O program")
	}
}

func grepLines(text string, needles ...string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				kept = append(kept, line)
			}
		}
	}
	return strings.Join(kept, "\n")
}

func TestPlistRegistersLinkSchemes(t *testing.T) {
	info := sampleInfo()
	info.URLSchemes = []string{"ssh", "telnet"}
	data, err := info.Plist()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"<key>CFBundleURLTypes</key>", "<key>CFBundleURLSchemes</key>", "<string>ssh</string>", "<string>telnet</string>"} {
		if !strings.Contains(text, want) {
			t.Errorf("plist lacks %q:\n%s", want, text)
		}
	}
	info.Extra = []KeyValue{{"CFBundleURLTypes", "x"}}
	if _, err := info.Plist(); err == nil {
		t.Error("an extra entry overrode CFBundleURLTypes")
	}
}

func TestPlistRegistersFileTypes(t *testing.T) {
	info := sampleInfo()
	info.FileTypes = []FileType{{Name: "Notes & lists", Extensions: []string{"note", "nt"}}}
	data, err := info.Plist()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"<key>CFBundleDocumentTypes</key>", "<string>Notes &amp; lists</string>", "<key>CFBundleTypeExtensions</key>", "<string>note</string>", "<string>nt</string>"} {
		if !strings.Contains(text, want) {
			t.Errorf("plist lacks %q:\n%s", want, text)
		}
	}
	if runtime.GOOS == "darwin" {
		if path, err := exec.LookPath("plutil"); err == nil {
			file := filepath.Join(t.TempDir(), "Info.plist")
			os.WriteFile(file, data, 0o644)
			if out, err := exec.Command(path, "-lint", file).CombinedOutput(); err != nil {
				t.Errorf("plutil: %v\n%s", err, out)
			}
		}
	}
	info.Extra = []KeyValue{{"CFBundleDocumentTypes", "x"}}
	if _, err := info.Plist(); err == nil {
		t.Error("an extra entry overrode CFBundleDocumentTypes")
	}
}
