// Package rpmpkg writes RPM binary packages (.rpm) for a desktop application:
// the program in /usr/bin, a menu entry, the MIME type and the hicolor icons.
// The file list is the one debpkg makes, so a .deb and an .rpm of the same
// application install the same files.
//
// An RPM is a lead, a signature header holding digests of what follows, the
// header (tags describing the package and its files) and a gzip-compressed cpio
// payload. The writer makes unsigned packages: it fills in the digests rpm
// verifies, so rpm -K --nosignature reports them intact. A full rpm -K also
// reports the missing signature, which is expected for an unsigned package. The output is
// deterministic: files are sorted, owners are root and the build time is
// tarball.SourceDate.
package rpmpkg

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/vivaui/viva/tools/debpkg"
	"github.com/vivaui/viva/tools/tarball"
)

// Package describes one RPM.
type Package struct {
	// App is the application: the same description the .deb is written from.
	// Its Depends are Debian names and are not used; see Requires.
	App debpkg.Package
	// Release is the release number, 1 when empty.
	Release string
	// License is the License tag; "Unspecified" when empty.
	License string
	// Requires are the capabilities the package needs, named as the target
	// distribution names them, including shared-library capabilities such as
	// libpango-1.0.so.0()(64bit).
	Requires []string
}

var (
	rpmVersionPattern = regexp.MustCompile(`^[0-9][A-Za-z0-9._+~^]*$`)
	rpmReleasePattern = regexp.MustCompile(`^[0-9][A-Za-z0-9._+~^]*$`)
)

// rpmArch is the architecture name rpm uses for a Viva architecture.
func rpmArch(arch string) (string, error) {
	switch arch {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	}
	return "", fmt.Errorf("rpmpkg: architecture %q must be amd64 or arm64", arch)
}

func (pkg Package) release() string {
	if pkg.Release == "" {
		return "1"
	}
	return pkg.Release
}

func (pkg Package) license() string {
	if pkg.License == "" {
		return "Unspecified"
	}
	return pkg.License
}

func (pkg Package) check() error {
	if _, err := debpkg.PackageName(pkg.App.Executable); err != nil {
		return err
	}
	if !rpmVersionPattern.MatchString(pkg.App.Version) {
		return fmt.Errorf("rpmpkg: %q is not an RPM version (it starts with a digit and has no hyphen)", pkg.App.Version)
	}
	if !rpmReleasePattern.MatchString(pkg.release()) {
		return fmt.Errorf("rpmpkg: %q is not an RPM release", pkg.release())
	}
	if _, err := rpmArch(pkg.App.Arch); err != nil {
		return err
	}
	if pkg.App.Name == "" {
		return errors.New("rpmpkg: the application needs a display name")
	}
	if len(pkg.App.Program) == 0 {
		return errors.New("rpmpkg: the program is empty")
	}
	if strings.ContainsAny(pkg.App.Comment, "\r\n") {
		return errors.New("rpmpkg: the comment must be one line")
	}
	if pkg.App.Maintainer == "" || strings.ContainsAny(pkg.App.Maintainer, "\r\n") {
		return errors.New("rpmpkg: the maintainer must be one line")
	}
	for _, requirement := range pkg.Requires {
		if requirement == "" || strings.ContainsAny(requirement, " ,\r\n\t") {
			return fmt.Errorf("rpmpkg: %q is not one capability name", requirement)
		}
	}
	return nil
}

// Name is the RPM name: the executable's package name as debpkg derives it.
func (pkg Package) Name() (string, error) {
	return debpkg.PackageName(pkg.App.Executable)
}

// FileName is the conventional file name, <name>-<version>-<release>.<arch>.rpm.
func (pkg Package) FileName() (string, error) {
	if err := pkg.check(); err != nil {
		return "", err
	}
	name, _ := pkg.Name()
	arch, _ := rpmArch(pkg.App.Arch)
	return fmt.Sprintf("%s-%s-%s.%s.rpm", name, pkg.App.Version, pkg.release(), arch), nil
}

// Write writes the .rpm.
func (pkg Package) Write(output io.Writer) error {
	if err := pkg.check(); err != nil {
		return err
	}
	name, _ := pkg.Name()
	arch, _ := rpmArch(pkg.App.Arch)
	files, err := pkg.App.Files()
	if err != nil {
		return err
	}
	modified := tarball.SourceDate()
	installed := installedFiles(files)
	nevr := name + "-" + pkg.App.Version + "-" + pkg.release()

	cpio := cpioArchive(installed, uint32(modified.Unix()))
	payload, err := tarball.Gzip(cpio)
	if err != nil {
		return err
	}
	// rpm checks the payload digest over the payload bytes as stored, that is
	// the compressed stream (rpm 6 reports a mismatch for the uncompressed cpio).
	payloadSum := sha256.Sum256(payload)
	header := encodeHeader(regionHeader, pkg.headerEntries(name, nevr, arch, installed, modified, hex.EncodeToString(payloadSum[:])))

	// The signature header covers the header and the compressed payload, as
	// rpm's own digests do.
	covered := append(append([]byte(nil), header...), payload...)
	sha1Sum := sha1.Sum(header)
	sha256Sum := sha256.Sum256(header)
	md5Sum := md5.Sum(covered)
	signature := padTo(encodeHeader(regionSignature, []entry{
		int32Tag(tagSigSize, []uint32{uint32(len(covered))}),
		int32Tag(tagSigPayloadSize, []uint32{uint32(len(cpio))}),
		binTag(tagSigMD5, md5Sum[:]),
		stringTag(tagSigSHA1, hex.EncodeToString(sha1Sum[:])),
		stringTag(tagSigSHA256, hex.EncodeToString(sha256Sum[:])),
	}), 8)

	for _, part := range [][]byte{leadBytes(nevr), signature, header, payload} {
		if _, err := output.Write(part); err != nil {
			return err
		}
	}
	return nil
}

// installedFile is one file of the payload with the values the header needs.
type installedFile struct {
	full string // absolute path, starting with /
	dir  string // the directory with a trailing slash
	base string
	data []byte
	mode uint16 // the regular-file type bits with the permission bits
}

// installedFiles sorts the files by absolute path. The header's file arrays
// and the payload both use this order, which rpm requires.
func installedFiles(files []tarball.File) []installedFile {
	sorted := append([]tarball.File(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	out := make([]installedFile, 0, len(sorted))
	for _, file := range sorted {
		full := "/" + file.Path
		out = append(out, installedFile{
			full: full,
			dir:  path.Dir(full) + "/",
			base: path.Base(full),
			data: file.Data,
			mode: uint16(0o100000 | (file.Mode & 0o7777)),
		})
	}
	return out
}

// headerEntries are the tags of the package header, sorted by tag when encoded.
func (pkg Package) headerEntries(name, nevr, arch string, files []installedFile, modified time.Time, payloadDigest string) []entry {
	count := len(files)
	var installedSize uint32
	sizes := make([]uint32, count)
	modes := make([]uint16, count)
	rdevs := make([]uint16, count)
	mtimes := make([]uint32, count)
	digests := make([]string, count)
	links := make([]string, count)
	flags := make([]uint32, count)
	verify := make([]uint32, count)
	users := make([]string, count)
	inodes := make([]uint32, count)
	devices := make([]uint32, count)
	langs := make([]string, count)
	dirIndexes := make([]uint32, count)
	baseNames := make([]string, count)
	var dirs []string
	dirIndex := map[string]uint32{}
	for i, file := range files {
		installedSize += uint32(len(file.data))
		sizes[i] = uint32(len(file.data))
		modes[i] = file.mode
		mtimes[i] = uint32(modified.Unix())
		sum := sha256.Sum256(file.data)
		digests[i] = hex.EncodeToString(sum[:])
		users[i] = "root"
		inodes[i] = uint32(i + 1)
		devices[i] = 1
		verify[i] = 0xffffffff
		if _, ok := dirIndex[file.dir]; !ok {
			dirIndex[file.dir] = uint32(len(dirs))
			dirs = append(dirs, file.dir)
		}
		dirIndexes[i] = dirIndex[file.dir]
		baseNames[i] = file.base
	}

	entries := []entry{
		stringTag(tagName, name),
		stringTag(tagVersion, pkg.App.Version),
		stringTag(tagRelease, pkg.release()),
		i18nTag(tagSummary, pkg.App.Comment),
		i18nTag(tagDescription, descriptionOf(pkg.App)),
		int32Tag(tagBuildTime, []uint32{uint32(modified.Unix())}),
		stringTag(tagBuildHost, "viva"),
		int32Tag(tagSize, []uint32{installedSize}),
		stringTag(tagLicense, pkg.license()),
		stringTag(tagPackager, pkg.App.Maintainer),
		i18nTag(tagGroup, "Applications/System"),
		stringTag(tagOS, "linux"),
		stringTag(tagArch, arch),
		stringTag(tagSourceRPM, nevr+".src.rpm"),
		int32Tag(tagFileSizes, sizes),
		int16Tag(tagFileModes, modes),
		int16Tag(tagFileRDevs, rdevs),
		int32Tag(tagFileMTimes, mtimes),
		stringArrayTag(tagFileDigests, digests),
		stringArrayTag(tagFileLinkTos, links),
		int32Tag(tagFileFlags, flags),
		int32Tag(tagFileVerifyFlags, verify),
		stringArrayTag(tagFileUserName, users),
		stringArrayTag(tagFileGroupName, users),
		int32Tag(tagFileDevices, devices),
		int32Tag(tagFileInodes, inodes),
		stringArrayTag(tagFileLangs, langs),
		int32Tag(tagDirIndexes, dirIndexes),
		stringArrayTag(tagBaseNames, baseNames),
		stringArrayTag(tagDirNames, dirs),
		stringArrayTag(tagProvideName, []string{name}),
		int32Tag(tagProvideFlags, []uint32{rpmSenseEqual}),
		stringArrayTag(tagProvideVersion, []string{strings.TrimPrefix(nevr, name+"-")}),
		stringTag(tagPayloadFormat, "cpio"),
		stringTag(tagPayloadCompressor, "gzip"),
		stringTag(tagPayloadFlags, "9"),
		int32Tag(tagFileDigestAlgo, []uint32{8}), // PGPHASHALGO_SHA256
		stringArrayTag(tagPayloadDigest, []string{payloadDigest}),
		int32Tag(tagPayloadDigestAlgo, []uint32{8}),
	}
	if pkg.App.Homepage != "" {
		entries = append(entries, stringTag(tagURL, pkg.App.Homepage))
	}
	if len(pkg.Requires) > 0 {
		entries = append(entries,
			stringArrayTag(tagRequireName, pkg.Requires),
			int32Tag(tagRequireFlags, make([]uint32, len(pkg.Requires))),
			stringArrayTag(tagRequireVersion, make([]string, len(pkg.Requires))),
		)
	}
	return entries
}

// descriptionOf is the package description: the extended text when there is
// one, otherwise the one-line comment (rpm needs a description).
func descriptionOf(app debpkg.Package) string {
	if text := strings.TrimSpace(app.Description); text != "" {
		return text
	}
	if app.Comment != "" {
		return app.Comment
	}
	return app.Name
}

// rpmSenseEqual is RPMSENSE_EQUAL: the provide's version must equal the one given.
const rpmSenseEqual = 8

// cpioArchive writes the payload as a newc cpio archive with rpm's "./" names,
// ending with the TRAILER!!! entry. Every file is owned by root and has one
// time, so the output is reproducible.
func cpioArchive(files []installedFile, mtime uint32) []byte {
	var out bytes.Buffer
	for i, file := range files {
		writeCPIOEntry(&out, uint32(i+1), uint32(file.mode), mtime, file.data, "."+file.full)
	}
	writeCPIOEntry(&out, 0, 0, mtime, nil, "TRAILER!!!")
	return out.Bytes()
}

// writeCPIOEntry writes one newc entry: "070701", thirteen 8-digit hex fields,
// the name with its NUL padded to 4 bytes from the header start, and the data
// padded to 4 bytes.
func writeCPIOEntry(out *bytes.Buffer, inode, mode, mtime uint32, data []byte, name string) {
	nameZ := name + "\x00"
	fields := []uint32{
		inode, mode, 0, 0, 1, mtime, uint32(len(data)),
		0, 0, 0, 0, uint32(len(nameZ)), 0,
	}
	out.WriteString("070701")
	for _, field := range fields {
		fmt.Fprintf(out, "%08X", field)
	}
	out.WriteString(nameZ)
	out.Write(make([]byte, padLen(110+len(nameZ), 4)))
	out.Write(data)
	out.Write(make([]byte, padLen(len(data), 4)))
}

// padLen is how many zero bytes bring n up to a multiple of align.
func padLen(n, align int) int {
	return (align - n%align) % align
}

// padTo appends zeros until the length is a multiple of align.
func padTo(data []byte, align int) []byte {
	return append(data, make([]byte, padLen(len(data), align))...)
}

// leadBytes is the 96-byte lead: the magic, the format version 3.0, a binary
// package, the signature type 5 (a header), Linux, and the name-version-release
// (66 bytes, NUL-terminated).
func leadBytes(nevr string) []byte {
	lead := make([]byte, 96)
	copy(lead, []byte{0xed, 0xab, 0xee, 0xdb, 3, 0})
	binary.BigEndian.PutUint16(lead[6:], 0)  // package type: binary
	binary.BigEndian.PutUint16(lead[8:], 1)  // archnum: not used by rpm to install
	copy(lead[10:76], nevr)                  // name, NUL-terminated
	binary.BigEndian.PutUint16(lead[76:], 1) // osnum: Linux
	binary.BigEndian.PutUint16(lead[78:], 5) // signature type: header
	return lead
}

// entry is one header tag: its value's type, element count and encoded bytes.
type entry struct {
	tag, typ, count uint32
	data            []byte
}

func stringTag(tag uint32, value string) entry {
	return entry{tag, typeString, 1, append([]byte(value), 0)}
}

func i18nTag(tag uint32, value string) entry {
	return entry{tag, typeI18N, 1, append([]byte(value), 0)}
}

func stringArrayTag(tag uint32, values []string) entry {
	var data []byte
	for _, value := range values {
		data = append(append(data, value...), 0)
	}
	return entry{tag, typeStringList, uint32(len(values)), data}
}

func binTag(tag uint32, value []byte) entry {
	return entry{tag, typeBin, uint32(len(value)), append([]byte(nil), value...)}
}

func int16Tag(tag uint32, values []uint16) entry {
	data := make([]byte, 2*len(values))
	for i, value := range values {
		binary.BigEndian.PutUint16(data[2*i:], value)
	}
	return entry{tag, typeInt16, uint32(len(values)), data}
}

func int32Tag(tag uint32, values []uint32) entry {
	data := make([]byte, 4*len(values))
	for i, value := range values {
		binary.BigEndian.PutUint32(data[4*i:], value)
	}
	return entry{tag, typeInt32, uint32(len(values)), data}
}

// alignment is the byte boundary a value of the type must start on in the
// header's data area.
func alignment(typ uint32) int {
	switch typ {
	case typeInt16:
		return 2
	case typeInt32:
		return 4
	}
	return 1
}

// encodeHeader writes a header section: the magic, the index count and the data
// size, the index records, then the data. The last index record is the region
// tag, whose 16-byte data repeats it, so rpm can find where the header ends.
func encodeHeader(region uint32, entries []entry) []byte {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].tag < entries[j].tag })
	var data bytes.Buffer
	type record struct{ tag, typ, offset, count uint32 }
	records := make([]record, 0, len(entries)+1)
	for _, item := range entries {
		for data.Len()%alignment(item.typ) != 0 {
			data.WriteByte(0)
		}
		records = append(records, record{item.tag, item.typ, uint32(data.Len()), item.count})
		data.Write(item.data)
	}
	// The trailer is the last 16 bytes of the data: a copy of the region record
	// whose offset is minus the record count, itself included. The region record
	// in the index points at that copy. Both match rpm's own headers.
	regionCount := len(records) + 1
	trailer := make([]byte, 16)
	binary.BigEndian.PutUint32(trailer[0:], region)
	binary.BigEndian.PutUint32(trailer[4:], typeBin)
	binary.BigEndian.PutUint32(trailer[8:], uint32(int32(-regionCount*16)))
	binary.BigEndian.PutUint32(trailer[12:], 16)
	records = append(records, record{region, typeBin, uint32(data.Len()), 16})
	// The region record sorts with the others: its tag (62 or 63) is lower than
	// every other tag, so rpm finds it first in the index.
	sort.SliceStable(records, func(i, j int) bool { return records[i].tag < records[j].tag })
	data.Write(trailer)

	var out bytes.Buffer
	out.Write([]byte{0x8e, 0xad, 0xe8, 0x01, 0, 0, 0, 0})
	binary.Write(&out, binary.BigEndian, uint32(len(records)))
	binary.Write(&out, binary.BigEndian, uint32(data.Len()))
	for _, item := range records {
		binary.Write(&out, binary.BigEndian, [4]uint32{item.tag, item.typ, item.offset, item.count})
	}
	out.Write(data.Bytes())
	return out.Bytes()
}

// Region tags: the main header and the signature header each end with one.
const (
	regionHeader    = 63 // RPMTAG_HEADERIMMUTABLE
	regionSignature = 62 // RPMTAG_HEADERSIGNATURES
)

// Tags, from rpm's rpmtag.h. Only the ones the writer uses are listed.
const (
	tagName              = 1000
	tagVersion           = 1001
	tagRelease           = 1002
	tagSummary           = 1004
	tagDescription       = 1005
	tagBuildTime         = 1006
	tagBuildHost         = 1007
	tagSize              = 1009
	tagLicense           = 1014
	tagPackager          = 1015
	tagGroup             = 1016
	tagURL               = 1020
	tagOS                = 1021
	tagArch              = 1022
	tagFileSizes         = 1028
	tagProvideName       = 1047
	tagFileModes         = 1030
	tagFileRDevs         = 1033
	tagFileMTimes        = 1034
	tagFileDigests       = 1035
	tagFileLinkTos       = 1036
	tagFileFlags         = 1037
	tagFileUserName      = 1039
	tagFileGroupName     = 1040
	tagSourceRPM         = 1044
	tagFileVerifyFlags   = 1045
	tagRequireFlags      = 1048
	tagRequireName       = 1049
	tagRequireVersion    = 1050
	tagFileDevices       = 1095
	tagFileInodes        = 1096
	tagFileLangs         = 1097
	tagProvideFlags      = 1112
	tagProvideVersion    = 1113
	tagDirIndexes        = 1116
	tagBaseNames         = 1117
	tagDirNames          = 1118
	tagPayloadFormat     = 1124
	tagPayloadCompressor = 1125
	tagPayloadFlags      = 1126
	tagFileDigestAlgo    = 5011
	tagPayloadDigest     = 5092
	tagPayloadDigestAlgo = 5093
)

// Tags of the signature header.
const (
	tagSigSHA1        = 269
	tagSigSHA256      = 273
	tagSigSize        = 1000
	tagSigMD5         = 1004
	tagSigPayloadSize = 1007
)

// Value types, from rpm's rpmtag.h.
const (
	typeInt16      = 3
	typeInt32      = 4
	typeString     = 6
	typeBin        = 7
	typeStringList = 8
	typeI18N       = 9
)
