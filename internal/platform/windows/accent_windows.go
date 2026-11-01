package windows

import (
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

var setWindowCompositionAttribute = user.NewProc("SetWindowCompositionAttribute")

// WCA_ACCENT_POLICY is undocumented. Keep its ABI and version-gated use here;
// Windows 11 22H2 and later use the documented system backdrop API instead.
type accentPolicy struct {
	State, Flags, GradientColor, AnimationID uint32
}

type compositionAttributeData struct {
	Attribute uint32
	Data      *accentPolicy
	Size      uintptr
}

func acrylicAccentPolicy(style platform.BackdropStyle) accentPolicy {
	if style.Kind != platform.BackdropAcrylic {
		return accentPolicy{} // ACCENT_DISABLED
	}
	// A nonzero alpha is required by legacy acrylic. The tint is AABBGGRR.
	color := uint32(0xccf2f2f2)
	if style.Dark {
		color = 0xcc202020
	}
	return accentPolicy{State: 4, GradientColor: color} // ACCENT_ENABLE_ACRYLICBLURBEHIND
}

func setAccentPolicy(window uintptr, policy accentPolicy) bool {
	if setWindowCompositionAttribute.Find() != nil {
		return false
	}
	data := compositionAttributeData{Attribute: 19, Data: &policy, Size: unsafe.Sizeof(policy)}
	ok, _, _ := setWindowCompositionAttribute.Call(window, uintptr(unsafe.Pointer(&data)))
	return ok != 0 // BOOL, not HRESULT.
}

func (state *windowState) applyAccentBackdrop(style platform.BackdropStyle) bool {
	frame := margins{Bottom: 1}
	if style.Kind == platform.BackdropAcrylic && !state.customFrame() {
		frame = margins{Left: -1, Right: -1, Top: -1, Bottom: -1}
	}
	extended, _, _ := dwmExtendFrame.Call(state.window, uintptr(unsafe.Pointer(&frame)))
	ok := setAccentPolicy(state.window, acrylicAccentPolicy(style))
	active := style.Kind == platform.BackdropAcrylic && ok && hresult(extended) == nil
	if !active && style.Kind == platform.BackdropAcrylic {
		setAccentPolicy(state.window, accentPolicy{})
		frame = margins{Bottom: 1}
		dwmExtendFrame.Call(state.window, uintptr(unsafe.Pointer(&frame)))
	}
	return active
}

// Missing settings retain Windows' default. Both material paths use this so
// OnBackdropChange also reports the user explicitly disabling transparency.
func accentTransparencyEnabled() bool {
	if regGetValue.Find() != nil {
		return true
	}
	subkey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	name, _ := syscall.UTF16PtrFromString("EnableTransparency")
	var value, size uint32 = 1, 4
	status, _, _ := regGetValue.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(subkey)), uintptr(unsafe.Pointer(name)),
		rrfRTRegDWord, 0, uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size)))
	return status != 0 || value != 0
}
