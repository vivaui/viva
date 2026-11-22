package semantic

// Role describes the accessible purpose of a node: the platform-neutral role an
// accessibility node carries to each platform's own.
type Role uint8

const (
	RoleApplication Role = iota + 1
	RoleGroup
	RoleButton
	RoleText
	RoleTextField
	RoleCheckbox
	RoleSlider
	RoleImage
	RoleLink
	RoleRadio
	RoleTab
	RoleMenuItem
	RoleCombobox
	RoleSwitch
	RoleMenu
	RoleMenubar
	RoleListBox
	RoleOption
	RoleGrid
	RoleGridCell
	RoleHeading
	RoleStatus
	RoleSeparator
	RoleProgress
)
