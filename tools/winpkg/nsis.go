package winpkg

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Installation scopes.
const (
	ScopeUser    = "user"
	ScopeMachine = "machine"
)

// Script is what the installer script is made from.
type Script struct {
	// Name is the display name; it names the install directory and the Start
	// Menu shortcut.
	Name    string
	Version string
	// Publisher defaults to Name.
	Publisher string
	// URL is the application's web page; empty leaves it out.
	URL string
	// ID is the reverse-DNS identifier; it names the uninstall registry key.
	ID string
	// Scope is ScopeUser (the default) or ScopeMachine.
	Scope string
	// Executable is the file name of the program once installed, ending .exe.
	Executable string
	// Program is the path of the built program the installer embeds.
	Program string
	// Icon is the path of an .ico file, or empty for none.
	Icon string
	// Output is the path of the installer to write.
	Output string
	// Arch is "amd64" or "arm64".
	Arch string
	// URLSchemes are the link schemes the program opens; the installer registers
	// each as a protocol that starts the program with the link as its argument.
	URLSchemes []string
	// FileTypes are the document types the program opens; the installer offers
	// the program for their extensions under "Open with", and passes the file's
	// path as the argument.
	FileTypes []FileType
}

// FileType is one kind of document: a description and its extensions without
// the dot.
type FileType struct {
	Name       string
	Extensions []string
}

var (
	idPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	versionPattern   = regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,3}$`)
	schemePattern    = regexp.MustCompile(`^[a-z][a-z0-9+.-]{0,31}$`)
	extensionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_+-]{0,15}$`)
	exePattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.exe$`)
)

// text checks one string that will be written between quotes and returns it
// escaped: $ is NSIS's escape character and a quote ends the string. A newline
// or other control character cannot be written inside a string, so it is an
// error rather than something to guess a meaning for.
func text(field, value string) (string, error) {
	if strings.IndexFunc(value, func(r rune) bool { return r < ' ' || r == 0x7f }) >= 0 {
		return "", fmt.Errorf("winpkg: %s %q cannot contain a newline or other control character", field, value)
	}
	value = strings.ReplaceAll(value, "$", "$$")
	value = strings.ReplaceAll(value, `"`, `$\"`)
	return value, nil
}

// numericVersion pads a version to the four numbers a Windows version resource
// holds.
func numericVersion(version string) (string, error) {
	parts := strings.Split(version, ".")
	for _, part := range parts {
		if number, err := strconv.Atoi(part); err != nil || number > 65535 {
			return "", fmt.Errorf("winpkg: version %q: each part must be a number up to 65535", version)
		}
	}
	for len(parts) < 4 {
		parts = append(parts, "0")
	}
	return strings.Join(parts, "."), nil
}

// Generate renders the NSIS script. The same Script gives the same bytes.
func (script Script) Generate() ([]byte, error) {
	name := script.Name
	switch {
	case name == "":
		return nil, errors.New("winpkg: the application needs a name")
	case strings.ContainsAny(name, `<>:"/\|?*`), strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return nil, fmt.Errorf("winpkg: the name %q is used as a directory name, so it cannot contain < > : \" / \\ | ? * or end in a dot or space", name)
	case !idPattern.MatchString(script.ID):
		return nil, fmt.Errorf("winpkg: the identifier %q must be letters, digits, . _ - (it names a registry key)", script.ID)
	case !versionPattern.MatchString(script.Version):
		return nil, fmt.Errorf("winpkg: the version %q must be up to four dot-separated numbers", script.Version)
	case !exePattern.MatchString(script.Executable):
		return nil, fmt.Errorf("winpkg: the executable %q must be a plain file name ending in .exe", script.Executable)
	case script.Program == "" || script.Output == "":
		return nil, errors.New("winpkg: the program and the installer paths are required")
	}
	for _, scheme := range script.URLSchemes {
		if !schemePattern.MatchString(scheme) {
			return nil, fmt.Errorf("winpkg: %q is not a link scheme", scheme)
		}
	}
	for _, kind := range script.FileTypes {
		if len(kind.Extensions) == 0 {
			return nil, fmt.Errorf("winpkg: the file type %q has no extension", kind.Name)
		}
		for _, extension := range kind.Extensions {
			if !extensionPattern.MatchString(extension) {
				return nil, fmt.Errorf("winpkg: %q is not a file extension", extension)
			}
		}
	}
	scope := script.Scope
	if scope == "" {
		scope = ScopeUser
	}
	if scope != ScopeUser && scope != ScopeMachine {
		return nil, fmt.Errorf("winpkg: unknown scope %q", script.Scope)
	}
	if script.Arch != "amd64" && script.Arch != "arm64" {
		return nil, fmt.Errorf("winpkg: unknown architecture %q; use amd64 or arm64", script.Arch)
	}
	numeric, err := numericVersion(script.Version)
	if err != nil {
		return nil, err
	}
	publisher := script.Publisher
	if publisher == "" {
		publisher = name
	}

	fields := map[string]string{}
	for field, value := range map[string]string{
		"name": name, "version": script.Version, "publisher": publisher, "url": script.URL, "id": script.ID,
		"executable": script.Executable, "program": script.Program, "icon": script.Icon, "output": script.Output,
	} {
		escaped, err := text(field, value)
		if err != nil {
			return nil, err
		}
		fields[field] = escaped
	}

	root, installDir, level := "HKCU", `$LOCALAPPDATA\Programs\${APP_NAME}`, "user"
	if scope == ScopeMachine {
		root, installDir, level = "HKLM", `$PROGRAMFILES64\${APP_NAME}`, "admin"
	}
	iconFile := strings.TrimSuffix(script.Executable, ".exe") + ".ico"

	var b strings.Builder
	line := func(format string, arguments ...any) { fmt.Fprintf(&b, format+"\n", arguments...) }

	line("; Generated by viva package. Edit viva.toml, not this file.")
	line("Unicode true")
	line("SetCompressor /SOLID lzma")
	line("ManifestDPIAware true")
	line("")
	line(`!define APP_NAME "%s"`, fields["name"])
	line(`!define APP_VERSION "%s"`, fields["version"])
	line(`!define APP_PUBLISHER "%s"`, fields["publisher"])
	if script.URL != "" {
		line(`!define APP_URL "%s"`, fields["url"])
	}
	line(`!define APP_ID "%s"`, fields["id"])
	line(`!define APP_EXE "%s"`, fields["executable"])
	line(`!define APP_ICON "%s"`, iconFile)
	line(`!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_ID}"`)
	line("")
	line(`!include "MUI2.nsh"`)
	line(`!include "LogicLib.nsh"`)
	line(`!include "x64.nsh"`)
	line(`!include "Sections.nsh"`)
	line("")
	line(`Name "${APP_NAME}"`)
	line(`OutFile "%s"`, fields["output"])
	line(`BrandingText "${APP_NAME} ${APP_VERSION}"`)
	line(`RequestExecutionLevel %s`, level)
	line(`InstallDir "%s"`, installDir)
	line("")
	line(`!define MUI_ABORTWARNING`)
	if script.Icon != "" {
		line(`!define MUI_ICON "%s"`, fields["icon"])
		line(`!define MUI_UNICON "%s"`, fields["icon"])
	}
	if scope == ScopeUser {
		line(`!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"`)
	}
	line(`!define MUI_FINISHPAGE_SHOWREADME`)
	line(`!define MUI_FINISHPAGE_SHOWREADME_TEXT "$(CREATE_DESKTOP_SHORTCUT)"`)
	line(`!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut`)
	line(`!insertmacro MUI_PAGE_WELCOME`)
	line(`!insertmacro MUI_PAGE_DIRECTORY`)
	line(`!insertmacro MUI_PAGE_INSTFILES`)
	line(`!insertmacro MUI_PAGE_FINISH`)
	line(`!insertmacro MUI_UNPAGE_CONFIRM`)
	line(`!insertmacro MUI_UNPAGE_INSTFILES`)
	line(`!insertmacro MUI_LANGUAGE "English"`)
	line(`!insertmacro MUI_LANGUAGE "SimpChinese"`)
	line("")
	line(`LangString CLOSE_APP ${LANG_ENGLISH} "Close ${APP_NAME} and try again."`)
	line(`LangString CLOSE_APP ${LANG_SIMPCHINESE} "请先关闭 ${APP_NAME}，然后重试。"`)
	line(`LangString CREATE_DESKTOP_SHORTCUT ${LANG_ENGLISH} "Create a desktop shortcut"`)
	line(`LangString CREATE_DESKTOP_SHORTCUT ${LANG_SIMPCHINESE} "创建桌面快捷方式"`)
	if script.Arch == "amd64" {
		line(`LangString NEEDS_ARCH ${LANG_ENGLISH} "${APP_NAME} needs 64-bit Windows."`)
		line(`LangString NEEDS_ARCH ${LANG_SIMPCHINESE} "${APP_NAME} 需要 64 位 Windows。"`)
	} else {
		line(`LangString NEEDS_ARCH ${LANG_ENGLISH} "This ${APP_NAME} is built for Windows on ARM64."`)
		line(`LangString NEEDS_ARCH ${LANG_SIMPCHINESE} "此 ${APP_NAME} 适用于 ARM64 版 Windows。"`)
	}
	line("")
	line(`VIProductVersion "%s"`, numeric)
	line(`VIFileVersion "%s"`, numeric)
	for _, language := range []string{"ENGLISH", "SIMPCHINESE"} {
		for _, pair := range [][2]string{
			{"ProductName", "${APP_NAME}"}, {"CompanyName", "${APP_PUBLISHER}"},
			{"FileDescription", "${APP_NAME} Setup"}, {"FileVersion", "${APP_VERSION}"}, {"ProductVersion", "${APP_VERSION}"},
		} {
			line(`VIAddVersionKey /LANG=${LANG_%s} "%s" "%s"`, language, pair[0], pair[1])
		}
	}
	line("")

	// The installer and the uninstaller each need their own copy of this
	// function, and a macro keeps the two the same.
	//
	// Windows refuses to delete or overwrite the image of a running program but
	// does allow renaming it: the process keeps running the old image and the
	// next launch gets the new one. Asking the user would end a silent install
	// (/S), where the message box answers itself with cancel, and any process
	// that outlives its window (an agent's MCP server, say) would block every
	// update, so move the file aside first and only ask when even that failed.
	line(`!macro EnsureClosed un`)
	line(`Function ${un}EnsureClosed`)
	line(`  retry:`)
	line(`  ${IfNot} ${FileExists} "$INSTDIR\${APP_EXE}"`)
	line(`    Return`)
	line(`  ${EndIf}`)
	line(`  ClearErrors`)
	line(`  Delete "$INSTDIR\${APP_EXE}"`)
	line(`  ${IfNot} ${Errors}`)
	line(`    Return`)
	line(`  ${EndIf}`)
	line(`  ; A running image can be renamed but not deleted, so move it aside.`)
	line(`  Push $1`)
	line(`  System::Call 'kernel32::GetTickCount()i .r1'`)
	line(`  ClearErrors`)
	line(`  Rename "$INSTDIR\${APP_EXE}" "$INSTDIR\${APP_EXE}.$1.old"`)
	line(`  ${IfNot} ${Errors}`)
	line(`    Pop $1`)
	line(`    Return`)
	line(`  ${EndIf}`)
	line(`  Pop $1`)
	line(`  MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "$(CLOSE_APP)" /SD IDCANCEL IDRETRY retry`)
	line(`  Abort`)
	line(`FunctionEnd`)
	line(`!macroend`)
	line(`!insertmacro EnsureClosed ""`)
	line(`!insertmacro EnsureClosed "un."`)
	line("")

	// The finish page runs after installation. The checkbox is MUI's "show
	// readme" option, so a silent install (/S) skips it and creates no shortcut.
	// The marker records that this installer created the shortcut once; the
	// uninstaller deletes only on that marker, so a same-named shortcut the
	// installer never made survives.
	line(`Function CreateDesktopShortcut`)
	if script.Icon != "" {
		line(`  CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_ICON}"`)
	} else {
		line(`  CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"`)
	}
	// A failed CreateShortcut leaves the error flag set; do not claim a shortcut
	// the uninstaller would then delete.
	line(`  ${IfNot} ${Errors}`)
	line(`    WriteRegDWORD %s "${UNINSTALL_KEY}" "DesktopShortcut" 1`, root)
	line(`  ${EndIf}`)
	line(`FunctionEnd`)
	line("")

	line(`Function .onInit`)
	if script.Arch == "amd64" {
		line(`  ${IfNot} ${RunningX64}`)
	} else {
		// A 32-bit installer running on ARM64 reports the native machine through
		// IsWow64Process2: 0xAA64 is 43620.
		line(`  System::Call 'kernel32::GetCurrentProcess()p.r1'`)
		line(`  System::Call 'kernel32::IsWow64Process2(p r1, *i .r2, *i .r3)i .r0'`)
		line(`  ${If} $0 == 0`)
		line(`  ${OrIf} $3 != 43620`)
	}
	line(`    MessageBox MB_OK|MB_ICONSTOP "$(NEEDS_ARCH)" /SD IDOK`)
	line(`    Abort`)
	line(`  ${EndIf}`)
	line(`  SetRegView 64`)
	if scope == ScopeMachine {
		line(`  SetShellVarContext all`)
	} else {
		line(`  SetShellVarContext current`)
	}
	line(`  ; Reinstalling goes where the application already is, unless /D= chose a place.`)
	line(`  ${If} $INSTDIR == "%s"`, installDir)
	line(`    ReadRegStr $0 %s "${UNINSTALL_KEY}" "InstallLocation"`, root)
	line(`    ${If} $0 != ""`)
	line(`      StrCpy $INSTDIR $0`)
	line(`    ${EndIf}`)
	line(`  ${EndIf}`)
	line(`FunctionEnd`)
	line("")
	line(`Function un.onInit`)
	line(`  SetRegView 64`)
	if scope == ScopeMachine {
		line(`  SetShellVarContext all`)
	} else {
		line(`  SetShellVarContext current`)
	}
	line(`FunctionEnd`)
	line("")

	line(`Section "${APP_NAME}" SecMain`)
	line(`  SectionIn RO`)
	line(`  Call EnsureClosed`)
	line(`  ; What an earlier update moved aside goes once nothing holds it.`)
	line(`  ClearErrors`)
	line(`  Delete "$INSTDIR\${APP_EXE}.*.old"`)
	line(`  SetOutPath "$INSTDIR"`)
	line(`  ; The program's modification time would otherwise be stored in the installer`)
	line(`  ; and make every build of the same program differ.`)
	line(`  SetDateSave off`)
	line(`  File "/oname=${APP_EXE}" "%s"`, fields["program"])
	if script.Icon != "" {
		line(`  File "/oname=${APP_ICON}" "%s"`, fields["icon"])
		line(`  CreateShortcut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}" "" "$INSTDIR\${APP_ICON}"`)
	} else {
		line(`  CreateShortcut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"`)
	}
	line(`  WriteUninstaller "$INSTDIR\Uninstall.exe"`)
	line(`  WriteRegStr %s "${UNINSTALL_KEY}" "DisplayName" "${APP_NAME}"`, root)
	line(`  WriteRegStr %s "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"`, root)
	line(`  WriteRegStr %s "${UNINSTALL_KEY}" "Publisher" "${APP_PUBLISHER}"`, root)
	if script.URL != "" {
		line(`  WriteRegStr %s "${UNINSTALL_KEY}" "URLInfoAbout" "${APP_URL}"`, root)
	}
	if script.Icon != "" {
		line(`  WriteRegStr %s "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\${APP_ICON}"`, root)
	} else {
		line(`  WriteRegStr %s "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\${APP_EXE}"`, root)
	}
	line(`  WriteRegStr %s "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'`, root)
	line(`  WriteRegStr %s "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'`, root)
	line(`  WriteRegStr %s "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"`, root)
	line(`  WriteRegDWORD %s "${UNINSTALL_KEY}" "NoModify" 1`, root)
	line(`  WriteRegDWORD %s "${UNINSTALL_KEY}" "NoRepair" 1`, root)
	for _, scheme := range script.URLSchemes {
		key := `Software\Classes\` + scheme
		line(`  WriteRegStr %s "%s" "" "URL:${APP_NAME}"`, root, key)
		line(`  WriteRegStr %s "%s" "URL Protocol" ""`, root, key)
		line(`  WriteRegStr %s "%s\DefaultIcon" "" '"$INSTDIR\${APP_EXE}",0'`, root, key)
		line(`  WriteRegStr %s "%s\shell\open\command" "" '"$INSTDIR\${APP_EXE}" "%%1"'`, root, key)
	}
	for _, kind := range script.FileTypes {
		name, err := text("file type", kind.Name)
		if err != nil {
			return nil, err
		}
		progID := script.ID + "." + kind.Extensions[0]
		key := `Software\Classes\` + progID
		line(`  WriteRegStr %s "%s" "" "%s"`, root, key, name)
		line(`  WriteRegStr %s "%s\DefaultIcon" "" '"$INSTDIR\${APP_EXE}",0'`, root, key)
		line(`  WriteRegStr %s "%s\shell\open\command" "" '"$INSTDIR\${APP_EXE}" "%%1"'`, root, key)
		for _, extension := range kind.Extensions {
			line(`  WriteRegStr %s "Software\Classes\.%s\OpenWithProgids" "%s" ""`, root, extension, progID)
		}
	}
	if len(script.FileTypes) > 0 || len(script.URLSchemes) > 0 {
		line(`  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'`)
	}
	line(`  SectionGetSize ${SecMain} $0`)
	line(`  WriteRegDWORD %s "${UNINSTALL_KEY}" "EstimatedSize" $0`, root)
	line(`SectionEnd`)
	line("")

	// Only what the installer wrote is removed; the directory goes if that
	// leaves it empty.
	line(`Section "Uninstall"`)
	line(`  Call un.EnsureClosed`)
	line(`  ClearErrors`)
	line(`  Delete "$INSTDIR\${APP_EXE}.*.old"`)
	line(`  Delete "$INSTDIR\${APP_EXE}"`)
	if script.Icon != "" {
		line(`  Delete "$INSTDIR\${APP_ICON}"`)
	}
	line(`  Delete "$SMPROGRAMS\${APP_NAME}.lnk"`)
	line(`  ReadRegDWORD $0 %s "${UNINSTALL_KEY}" "DesktopShortcut"`, root)
	line(`  ${If} $0 == 1`)
	line(`    Delete "$DESKTOP\${APP_NAME}.lnk"`)
	line(`  ${EndIf}`)
	line(`  Delete "$INSTDIR\Uninstall.exe"`)
	line(`  RMDir "$INSTDIR"`)
	for _, scheme := range script.URLSchemes {
		line(`  DeleteRegKey %s "Software\Classes\%s"`, root, scheme)
	}
	for _, kind := range script.FileTypes {
		progID := script.ID + "." + kind.Extensions[0]
		for _, extension := range kind.Extensions {
			line(`  DeleteRegValue %s "Software\Classes\.%s\OpenWithProgids" "%s"`, root, extension, progID)
		}
		line(`  DeleteRegKey %s "Software\Classes\%s"`, root, progID)
	}
	if len(script.FileTypes) > 0 || len(script.URLSchemes) > 0 {
		line(`  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'`)
	}
	line(`  DeleteRegKey %s "${UNINSTALL_KEY}"`, root)
	line(`SectionEnd`)
	return []byte(b.String()), nil
}
