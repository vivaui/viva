//go:build windows

package windows

import (
	"bytes"
	"io"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// promiseFixture is a file and a folder, as a promised drag flattens them.
func promiseFixture(t *testing.T) []platform.FilePromise {
	t.Helper()
	file := func(name, value string) platform.FilePromise {
		return platform.FilePromise{Name: name, Size: int64(len(value)), Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(value)), nil
		}}
	}
	return []platform.FilePromise{
		file("notes.txt", "hello world"),
		{Name: "docs", Directory: true, List: func() ([]platform.FilePromise, error) {
			return []platform.FilePromise{file("guide.md", "guide")}, nil
		}},
	}
}

// newPromiseObject builds the data object of a promised drag with format ids
// that need no registration: the tests ask for these, not the shell's names.
func newPromiseObject(t *testing.T, files []platform.FilePromise) (*fileDataObject, uintptr) {
	t.Helper()
	items, err := promiseItems(files)
	if err != nil {
		t.Fatal(err)
	}
	dataTableOnce.Do(newFileDataTable)
	data := &fileDataObject{table: &dataTable, refs: 1}
	data.promise = &promiseDrag{items: items, descriptor: 0x9001, contents: 0x9002}
	pinPromiseDrag(data)
	t.Cleanup(func() { unpinPromiseDrag(data) })
	return data, uintptr(unsafe.Pointer(data))
}

func descriptorFormat(id uint16) formatEtc {
	return formatEtc{Format: id, Aspect: dvAspectContent, Index: lindexNone, Medium: tymedHGlobal}
}

func contentsFormat(id uint16, index int32) formatEtc {
	return formatEtc{Format: id, Aspect: dvAspectContent, Index: index, Medium: tymedIStream}
}

// TestThePromiseDataObjectDescribesTheWholeTree checks the group descriptor the
// shell reads to create the files: the names, folders, sizes and attributes.
func TestThePromiseDataObjectDescribesTheWholeTree(t *testing.T) {
	data, object := newPromiseObject(t, promiseFixture(t))
	data.promise.descriptor = 0x9001
	defer runtime.KeepAlive(data)
	format := descriptorFormat(0x9001)
	if status := uint32(comCall1(object, 5, uintptr(unsafe.Pointer(&format)))); status != sOK {
		t.Fatalf("QueryGetData(FileGroupDescriptor) = %#x, want S_OK", status)
	}
	var medium storageMedium
	if status := uint32(comCall2(object, 3, uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)))); status != sOK {
		t.Fatalf("GetData(FileGroupDescriptor) = %#x, want S_OK", status)
	}
	defer releaseStgMedium.Call(uintptr(unsafe.Pointer(&medium)))
	if medium.Medium != tymedHGlobal || medium.Handle == 0 {
		t.Fatalf("the descriptor medium is %+v, want memory the target owns", medium)
	}
	memory, _, _ := globalLock.Call(medium.Handle)
	if memory == 0 {
		t.Fatal("the descriptor block cannot be locked")
	}
	defer globalUnlock.Call(medium.Handle)
	block := unsafe.Slice((*byte)(pointer(memory)), 4+3*int(unsafe.Sizeof(fileDescriptorW{})))
	items := *(*uint32)(unsafe.Pointer(&block[0]))
	if items != 3 {
		t.Fatalf("the group names %d items, want 3 (notes.txt, docs, docs\\guide.md)", items)
	}
	if size := unsafe.Sizeof(fileDescriptorW{}); size != 592 {
		t.Fatalf("FILEDESCRIPTORW is %d bytes, want 592", size)
	}
	descriptors := unsafe.Slice((*fileDescriptorW)(unsafe.Pointer(&block[4])), int(items))
	names := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		names = append(names, syscall.UTF16ToString(descriptor.Name[:]))
	}
	if want := []string{"notes.txt", "docs", `docs\guide.md`}; !slices.Equal(names, want) {
		t.Fatalf("the group names %q, want %q", names, want)
	}
	if got := syscall.UTF16ToString(descriptors[0].Name[:]); got != "notes.txt" {
		t.Fatalf("the first name is %q", got)
	}
	if descriptors[0].Flags&fdFileSize == 0 || descriptors[0].SizeLow != 11 || descriptors[0].Attributes != fileAttributeNormal {
		t.Errorf("the file descriptor is %+v, want 11 bytes and a normal attribute", descriptors[0])
	}
	if descriptors[1].Attributes != fileAttributeDirectory || descriptors[1].Flags&fdFileSize != 0 {
		t.Errorf("the folder descriptor is %+v, want a directory without a size", descriptors[1])
	}
}

// TestThePromiseStreamProducesTheFileAsTheTargetReads checks the IStream the
// shell copies from: its size, its bytes, its end, and that a folder has none.
func TestThePromiseStreamProducesTheFileAsTheTargetReads(t *testing.T) {
	data, object := newPromiseObject(t, promiseFixture(t))
	data.promise.contents = 0x9002
	defer runtime.KeepAlive(data)
	format := contentsFormat(0x9002, 0)
	var medium storageMedium
	if status := uint32(comCall2(object, 3, uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)))); status != sOK {
		t.Fatalf("GetData(FileContents, 0) = %#x, want S_OK", status)
	}
	if medium.Medium != tymedIStream || medium.Handle == 0 {
		t.Fatalf("the content medium is %+v, want a stream", medium)
	}
	stream := medium.Handle
	if size := unsafe.Sizeof(statStg{}); size != 80 {
		t.Fatalf("STATSTG is %d bytes, want 80", size)
	}
	var stat statStg
	if status := uint32(comCall2(stream, 12, uintptr(unsafe.Pointer(&stat)), 0)); status != sOK {
		t.Fatalf("Stat = %#x, want S_OK", status)
	}
	if stat.Type != stgtYStream || stat.Size != 11 {
		t.Errorf("Stat reports type %d size %d, want a stream of 11 bytes", stat.Type, stat.Size)
	}
	buffer := make([]byte, 8)
	var read uint32
	if status := uint32(comCall3(stream, 3, uintptr(unsafe.Pointer(&buffer[0])), 8, uintptr(unsafe.Pointer(&read)))); status != sOK || read != 8 {
		t.Fatalf("Read(8) = %#x with %d bytes, want S_OK with 8", status, read)
	}
	if string(buffer) != "hello wo" {
		t.Fatalf("the first read is %q", buffer)
	}
	rest := make([]byte, 8)
	if status := uint32(comCall3(stream, 3, uintptr(unsafe.Pointer(&rest[0])), 8, uintptr(unsafe.Pointer(&read)))); status != sFalse || read != 3 {
		t.Fatalf("Read at the end = %#x with %d bytes, want S_FALSE with 3", status, read)
	}
	if !bytes.Equal(rest[:3], []byte("rld")) {
		t.Fatalf("the last bytes are %q, want \"rld\"", rest[:3])
	}
	if status := uint32(comCall3(stream, 3, uintptr(unsafe.Pointer(&rest[0])), 8, uintptr(unsafe.Pointer(&read)))); status != sFalse || read != 0 {
		t.Fatalf("Read past the end = %#x with %d bytes, want S_FALSE with 0", status, read)
	}
	comCall0(stream, 2) // Release, as the shell does with the medium
	if status := uint32(comCall2(object, 3, uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)))); status != sOK || medium.Handle == 0 {
		t.Fatalf("a second GetData = %#x with %+v, want a fresh stream", status, medium)
	}
	var second storageMedium = medium
	releaseStgMedium.Call(uintptr(unsafe.Pointer(&second)))
	// A folder is described, not streamed.
	folder := contentsFormat(0x9002, 1)
	if status := uint32(comCall2(object, 3, uintptr(unsafe.Pointer(&folder)), uintptr(unsafe.Pointer(&medium)))); status != dvEFormatEtc {
		t.Errorf("GetData for a folder = %#x, want DV_E_FORMATETC", status)
	}
}

// TestThePromiseEnumeratorListsBothFormats checks what OLE's standard
// marshaling reads: the descriptor and the contents format, and nothing more.
func TestThePromiseEnumeratorListsBothFormats(t *testing.T) {
	data, object := newPromiseObject(t, promiseFixture(t))
	defer runtime.KeepAlive(data)
	var enum uintptr
	if status := uint32(comCall2(object, 8, dataDirGet, uintptr(unsafe.Pointer(&enum)))); status != sOK || enum == 0 {
		t.Fatalf("EnumFormatEtc = %#x with %#x, want an enumerator", status, enum)
	}
	var offered [2]formatEtc
	var fetched uint32
	if status := uint32(comCall3(enum, 3, 2, uintptr(unsafe.Pointer(&offered[0])), uintptr(unsafe.Pointer(&fetched)))); status != sOK || fetched != 2 {
		t.Fatalf("Next(2) = %#x with %d formats, want S_OK with 2", status, fetched)
	}
	if offered[0].Format != 0x9001 || offered[0].Medium != tymedHGlobal {
		t.Errorf("the first format is %+v, want the group descriptor", offered[0])
	}
	if offered[1].Format != 0x9002 || offered[1].Medium != tymedIStream {
		t.Errorf("the second format is %+v, want file contents as a stream", offered[1])
	}
	if status := uint32(comCall3(enum, 3, 1, uintptr(unsafe.Pointer(&offered[0])), uintptr(unsafe.Pointer(&fetched)))); status != sFalse || fetched != 0 {
		t.Errorf("Next after the end = %#x with %d, want S_FALSE with 0", status, fetched)
	}
}

// TestPromiseNamesCheckTheShellLimit keeps a name the descriptor cannot hold
// from starting a drag that would fail when the shell reads it.
func TestPromiseNamesCheckTheShellLimit(t *testing.T) {
	long := strings.Repeat("a", maxDescriptorName)
	files := []platform.FilePromise{{Name: long, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("")), nil
	}}}
	if _, err := promiseItems(files); err == nil {
		t.Fatal("a name longer than the descriptor was accepted")
	}
}
