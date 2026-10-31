package windows

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	wmBackdropPower       = 0x0218 // WM_POWERBROADCAST
	wmBackdropTheme       = 0x031A // WM_THEMECHANGED
	wmBackdropComposition = 0x031E // WM_DWMCOMPOSITIONCHANGED
)

type backdropPolicy struct {
	highContrast, transparencyDisabled, batterySaver bool
}

func (policy backdropPolicy) allows(kind platform.BackdropKind) bool {
	return !policy.highContrast && !policy.transparencyDisabled &&
		!(policy.batterySaver && kind == platform.BackdropAcrylic)
}

// Read on creation and system notifications, not on every paint. The legacy
// accent API does not provide the policy fallback of Windows' modern materials.
func readBackdropPolicy() backdropPolicy {
	return backdropPolicy{
		highContrast: HighContrast(), transparencyDisabled: !accentTransparencyEnabled(),
		batterySaver: backdropBatterySaver(),
	}
}

var getBackdropPowerStatus = kernel.NewProc("GetSystemPowerStatus")
var registerBackdropPower = user.NewProc("RegisterPowerSettingNotification")
var unregisterBackdropPower = user.NewProc("UnregisterPowerSettingNotification")

// GUID_POWER_SAVING_STATUS, available since Windows 10.
var backdropPowerSavingGUID = guid{0xe00958c0, 0xc213, 0x4ace, [8]byte{0xac, 0x77, 0xfe, 0xcc, 0xed, 0x2e, 0xee, 0xa5}}

func backdropBatterySaver() bool {
	if getBackdropPowerStatus.Find() != nil {
		return false
	}
	var status struct {
		ACLineStatus, BatteryFlag, BatteryLifePercent, SystemStatusFlag byte
		BatteryLifeTime, BatteryFullLifeTime                            uint32
	}
	ok, _, _ := getBackdropPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	return ok != 0 && status.SystemStatusFlag == 1
}

func (state *windowState) watchBackdropPower() {
	if state.backdropPowerNotification != 0 || state.config.WindowBackdrop == nil || nativeBackdropAPI() == backdropUnavailable ||
		registerBackdropPower.Find() != nil || unregisterBackdropPower.Find() != nil {
		return
	}
	state.backdropPowerNotification, _, _ = registerBackdropPower.Call(state.window,
		uintptr(unsafe.Pointer(&backdropPowerSavingGUID)), 0) // DEVICE_NOTIFY_WINDOW_HANDLE
}

func (state *windowState) closeBackdropPower() {
	if state.backdropPowerNotification != 0 {
		unregisterBackdropPower.Call(state.backdropPowerNotification)
		state.backdropPowerNotification = 0
	}
}
