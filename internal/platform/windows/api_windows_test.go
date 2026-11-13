package windows

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestNativeStructureABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Fatal("only Windows amd64 and arm64 are supported")
	}
	for _, layout := range []struct {
		name string
		size uintptr
		want uintptr
	}{
		{"GUID", unsafe.Sizeof(guid{}), 16},
		{"POINT", unsafe.Sizeof(point{}), 8},
		{"RECT", unsafe.Sizeof(rect{}), 16},
		{"MSG", unsafe.Sizeof(message{}), 48},
		{"PAINTSTRUCT", unsafe.Sizeof(paintStruct{}), 72},
		{"WNDCLASSEXW", unsafe.Sizeof(windowClass{}), 80},
		{"CREATESTRUCTW", unsafe.Sizeof(createStruct{}), 80},
		{"TRACKMOUSEEVENT", unsafe.Sizeof(trackMouse{}), 24},
		{"COMPOSITIONFORM", unsafe.Sizeof(compositionForm{}), 28},
		{"COMDLG_FILTERSPEC", unsafe.Sizeof(dialogFilter{}), 16},
		{"NOTIFYICONDATAW", unsafe.Sizeof(notifyIconData{}), 976},
		{"NOTIFYICONIDENTIFIER", unsafe.Sizeof(notifyIconIdentifier{}), 40},
		{"ICONINFO", unsafe.Sizeof(iconInfo{}), 32},
		{"SHFILEINFOW", unsafe.Sizeof(shFileInfo{}), 696},
		{"BITMAP", unsafe.Sizeof(gdiBitmap{}), 32},
		{"MONO BITMAPINFO", unsafe.Sizeof(monoBitmapInfo{}), 48},
		{"BITMAPV4HEADER", unsafe.Sizeof(bitmapV4Header{}), 108},
		{"MONITORINFO", unsafe.Sizeof(monitorInfo{}), 40},
		{"D3D11_BUFFER_DESC", unsafe.Sizeof(bufferDesc{}), 24},
		{"D3D11_TEXTURE2D_DESC", unsafe.Sizeof(textureDesc{}), 44},
		{"D3D11_SUBRESOURCE_DATA", unsafe.Sizeof(resourceData{}), 16},
		{"D3D11_SHADER_RESOURCE_VIEW_DESC", unsafe.Sizeof(resourceViewDesc{}), 24},
		{"DXGI_SWAP_CHAIN_DESC1", unsafe.Sizeof(swapChainDesc1{}), 48},
		{"D3D11_RENDER_TARGET_BLEND_DESC", unsafe.Sizeof(renderBlend{}), 32},
		{"D3D11_BLEND_DESC", unsafe.Sizeof(blendDesc{}), 264},
		{"D3D11_RASTERIZER_DESC", unsafe.Sizeof(rasterDesc{}), 40},
		{"D3D11_SAMPLER_DESC", unsafe.Sizeof(samplerDesc{}), 52},
		{"D3D11_VIEWPORT", unsafe.Sizeof(viewport{}), 24},
		{"D3D11_BOX", unsafe.Sizeof(box{}), 24},
		{"BITMAPINFOHEADER", unsafe.Sizeof(bitmapInfoHeader{}), 40},
		{"DWRITE_LINE_METRICS", unsafe.Sizeof(lineMetrics{}), 24},
		{"DWRITE_TEXT_METRICS", unsafe.Sizeof(textMetrics{}), 36},
		{"rectangle shader instance", unsafe.Sizeof(rectangleInstance{}), rectangleInstanceBytes},
		{"frame shader constants", unsafe.Sizeof(frameConstants{}), 64},
	} {
		if layout.size != layout.want {
			t.Errorf("%s: size %d, want %d", layout.name, layout.size, layout.want)
		}
	}
	if unsafe.Offsetof(swapChainDesc1{}.Scaling) != 32 || unsafe.Offsetof(windowClass{}.Name) != 64 || unsafe.Offsetof(message{}.First) != 16 || unsafe.Offsetof(resourceData{}.Pitch) != 8 || unsafe.Offsetof(notifyIconData{}.Info) != 304 || unsafe.Offsetof(notifyIconData{}.BalloonIcon) != 968 {
		t.Fatal("native member alignment mismatch")
	}
	wantTextureIID := guid{0x6f15aaf2, 0xd208, 0x4e89, [8]byte{0x9a, 0xb4, 0x48, 0x95, 0x35, 0xd3, 0x4f, 0x9c}}
	if textureIID != wantTextureIID {
		t.Fatal("ID3D11Texture2D GUID mismatch")
	}
	if offset := unsafe.Offsetof(frameConstants{}.InstanceOffset); offset != 8 {
		t.Fatalf("frame shader instance offset = %d, want 8", offset)
	}
}

func TestCopyUTF16Bounds(t *testing.T) {
	var destination [4]uint16
	copyUTF16(destination[:], "abcdef")
	if destination[3] != 0 || syscall.UTF16ToString(destination[:]) != "abc" {
		t.Fatalf("truncated UTF-16 = %#v", destination)
	}
}

func TestNotificationIconCarriesABalloonWithoutShowingATrayEntry(t *testing.T) {
	data := notificationIconData(42, 7, "Motty", "Motty is still running")
	if data.Flags&notifyInfo == 0 || data.Flags&notifyState == 0 {
		t.Fatalf("notification flags = %#x", data.Flags)
	}
	if data.State != notifyHidden || data.StateMask != notifyHidden {
		t.Fatalf("notification state = %#x mask=%#x", data.State, data.StateMask)
	}
	if syscall.UTF16ToString(data.Tip[:]) != "Motty" ||
		syscall.UTF16ToString(data.InfoTitle[:]) != "Motty" ||
		syscall.UTF16ToString(data.Info[:]) != "Motty is still running" {
		t.Fatalf("notification text = tip %q title %q body %q",
			syscall.UTF16ToString(data.Tip[:]), syscall.UTF16ToString(data.InfoTitle[:]), syscall.UTF16ToString(data.Info[:]))
	}
}

func TestHRESULT(t *testing.T) {
	if hresult(0) != nil || hresult(1) != nil || hresult(0x7fffffff) != nil {
		t.Fatal("successful HRESULT rejected")
	}
	if hresult(0x80004005) == nil || hresult(0xffffffff80004005) == nil {
		t.Fatal("failed HRESULT accepted")
	}
}

func TestCOMCallKeepsPointersAlive(t *testing.T) {
	callback := syscall.NewCallback(func(object, destination uintptr) uintptr {
		runtime.GC()
		*(*uint64)(pointer(destination)) = 0x123456789abcdef0
		return 1
	})
	table := [1]uintptr{callback}
	object := struct{ Table *uintptr }{Table: &table[0]}
	var output uint64
	status := comCall(uintptr(unsafe.Pointer(&object)), 0, uintptr(unsafe.Pointer(&output)))
	if status != 1 || output != 0x123456789abcdef0 {
		t.Fatalf("COM call lost Go output pointer: %#x, %#x", status, output)
	}
	runtime.KeepAlive(table)
}

func TestCOMCallArgumentLimit(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("overlong COM call was not rejected")
		}
	}()
	comCall(0, 0, make([]uintptr, 16)...)
}
