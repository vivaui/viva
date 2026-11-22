package base

// Kind is what an element is: a text, a container, or one of the controls.
type Kind uint8

const (
	KindContainer Kind = iota
	KindText
	KindSpacer
	KindButton
	KindCheckbox
	KindSwitch
	KindIcon
	KindSlot
	KindInput
	KindInputGroup
	KindTextarea
	KindSegmented
	KindTable
	KindTabs
	KindSelect
	KindProgress
	KindSpinner
	KindSplitter
	KindSlider
	KindImage
	KindSplitButton
	KindBadge
	KindToggle
	KindPagination
)

// Direction is how a container lays its children out.
type Direction uint8

const (
	DirectionColumn Direction = iota
	DirectionRow
	DirectionStack
	DirectionGrid
	DirectionCustom // a Layout places the children itself
	DirectionFlow   // a row that wraps onto more lines
)

// DimKind is how a size is chosen: automatic, pixels, fill or percent.
type DimKind uint8

const (
	DimAuto DimKind = iota
	DimPx
	DimFill
	DimPercent
)
