package base

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/vivaui/viva/field"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/window"
)

// GetLayer reads the layer of AlertDialogDecl for the styled painting layer.
func (x *AlertDialogDecl) GetLayer() uint16 { return x.layer }

// GetSpec reads the spec of AlertDialogDecl for the styled painting layer.
func (x *AlertDialogDecl) GetSpec() AlertDialogSpec { return x.spec }

// GetF reads the f of Box for the styled painting layer.
func (x Box) GetF() *Frame { return x.f }

// GetGen reads the gen of Box for the styled painting layer.
func (x Box) GetGen() uint32 { return x.gen }

// GetI reads the i of Box for the styled painting layer.
func (x Box) GetI() int32 { return x.i }

// GetEmpty reads the empty of ColorPickerLook for the styled painting layer.
func (x *ColorPickerLook) GetEmpty() bool { return x.empty }

// GetIcon reads the icon of ColorPickerLook for the styled painting layer.
func (x *ColorPickerLook) GetIcon() *gfx.Icon { return x.icon }

// GetLabels reads the labels of ColorPickerLook for the styled painting layer.
func (x *ColorPickerLook) GetLabels() ColorPickerLabels { return x.labels }

// GetRound reads the round of ColorPickerLook for the styled painting layer.
func (x *ColorPickerLook) GetRound() bool { return x.round }

// GetApplied reads the applied of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetApplied() bool { return x.applied }

// GetDraft reads the draft of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetDraft() ColorPickerValue { return x.draft }

// GetHandlers reads the handlers of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetHandlers() Handlers { return x.handlers }

// GetHex reads the hex of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetHex() Field { return x.hex }

// GetHexEditor reads the hexEditor of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetHexEditor() text.Editor { return x.hexEditor }

// GetInvalid reads the invalid of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetInvalid() bool { return x.invalid }

// GetLayout reads the layout of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetLayout() ColorPickerLayout { return x.layout }

// GetOpen reads the open of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetOpen() bool { return x.open }

// GetRestoreFocus reads the restoreFocus of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetRestoreFocus() bool { return x.restoreFocus }

// GetSeen reads the seen of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetSeen() string { return x.seen }

// GetTarget reads the target of ColorPickerState for the styled painting layer.
func (x *ColorPickerState) GetTarget() *Color { return x.target }

// GetClearLabel reads the clearLabel of ComboState for the styled painting layer.
func (x *ComboState) GetClearLabel() string { return x.clearLabel }

// GetClearLabelOf reads the clearLabelOf of ComboState for the styled painting layer.
func (x *ComboState) GetClearLabelOf() string { return x.clearLabelOf }

// GetClearable reads the clearable of ComboState for the styled painting layer.
func (x *ComboState) GetClearable() bool { return x.clearable }

// GetCreatePrefix reads the createPrefix of ComboState for the styled painting layer.
func (x *ComboState) GetCreatePrefix() string { return x.createPrefix }

// GetCreated reads the created of ComboState for the styled painting layer.
func (x *ComboState) GetCreated() string { return x.created }

// GetDescs reads the descs of ComboState for the styled painting layer.
func (x *ComboState) GetDescs() []ComboboxOption { return x.descs }

// GetEmpty reads the empty of ComboState for the styled painting layer.
func (x *ComboState) GetEmpty() string { return x.empty }

// GetF reads the f of ComboState for the styled painting layer.
func (x *ComboState) GetF() *Frame { return x.f }

// GetGen reads the gen of ComboState for the styled painting layer.
func (x *ComboState) GetGen() uint32 { return x.gen }

// GetIds reads the ids of ComboState for the styled painting layer.
func (x *ComboState) GetIds() comboIDs { return x.ids }

// GetInput reads the input of ComboState for the styled painting layer.
func (x *ComboState) GetInput() window.TextInput { return x.input }

// GetKeys reads the keys of ComboState for the styled painting layer.
func (x *ComboState) GetKeys() []string { return x.keys }

// GetLabels reads the labels of ComboState for the styled painting layer.
func (x *ComboState) GetLabels() []string { return x.labels }

// GetLayer reads the layer of ComboState for the styled painting layer.
func (x *ComboState) GetLayer() uint16 { return x.layer }

// GetList reads the list of ComboState for the styled painting layer.
func (x *ComboState) GetList() SingleSelectState { return x.list }

// GetOpts reads the opts of ComboState for the styled painting layer.
func (x *ComboState) GetOpts() []SingleSelectOption { return x.opts }

// GetPaint reads the paint of ComboState for the styled painting layer.
func (x *ComboState) GetPaint() func(*DrawContext) { return x.paint }

// GetPlaceholder reads the placeholder of ComboState for the styled painting layer.
func (x *ComboState) GetPlaceholder() string { return x.placeholder }

// GetScratch reads the scratch of ComboState for the styled painting layer.
func (x *ComboState) GetScratch() []ComboboxOption { return x.scratch }

// GetSynced reads the synced of ComboState for the styled painting layer.
func (x *ComboState) GetSynced() string { return x.synced }

// GetF reads the f of Ctx for the styled painting layer.
func (x *Ctx) GetF() *Frame { return x.f }

// GetNextSet reads the nextSet of Ctx for the styled painting layer.
func (x *Ctx) GetNextSet() bool { return x.nextSet }

// GetPage reads the page of Ctx for the styled painting layer.
func (x *Ctx) GetPage() *Page { return x.page }

// GetStateSeq reads the stateSeq of Ctx for the styled painting layer.
func (x *Ctx) GetStateSeq() map[stateName]uint32 { return x.stateSeq }

// GetBody reads the body of DialogDecl for the styled painting layer.
func (x *DialogDecl) GetBody() int32 { return x.body }

// GetFooter reads the footer of DialogDecl for the styled painting layer.
func (x *DialogDecl) GetFooter() int32 { return x.footer }

// GetHeader reads the header of DialogDecl for the styled painting layer.
func (x *DialogDecl) GetHeader() int32 { return x.header }

// GetLayer reads the layer of DialogDecl for the styled painting layer.
func (x *DialogDecl) GetLayer() uint16 { return x.layer }

// GetSpec reads the spec of DialogDecl for the styled painting layer.
func (x *DialogDecl) GetSpec() DialogSpec { return x.spec }

// GetActive reads the active of DragState for the styled painting layer.
func (x *DragState) GetActive() bool { return x.active }

// GetAt reads the at of DragState for the styled painting layer.
func (x *DragState) GetAt() gfx.Point { return x.at }

// GetCancelled reads the cancelled of DragState for the styled painting layer.
func (x *DragState) GetCancelled() bool { return x.cancelled }

// GetCandidate reads the candidate of DragState for the styled painting layer.
func (x *DragState) GetCandidate() gfx.ID { return x.candidate }

// GetEpoch reads the epoch of DragState for the styled painting layer.
func (x *DragState) GetEpoch() uint32 { return x.epoch }

// GetGen reads the gen of DragState for the styled painting layer.
func (x *DragState) GetGen() uint32 { return x.gen }

// GetPressed reads the pressed of DragState for the styled painting layer.
func (x *DragState) GetPressed() bool { return x.pressed }

// GetSource reads the source of DragState for the styled painting layer.
func (x *DragState) GetSource() gfx.ID { return x.source }

// GetSources reads the sources of DragState for the styled painting layer.
func (x *DragState) GetSources() []dragSource { return x.sources }

// GetStart reads the start of DragState for the styled painting layer.
func (x *DragState) GetStart() gfx.Point { return x.start }

// GetTarget reads the target of DragState for the styled painting layer.
func (x *DragState) GetTarget() gfx.ID { return x.target }

// GetTargets reads the targets of DragState for the styled painting layer.
func (x *DragState) GetTargets() []dragTarget { return x.targets }

// GetClose reads the close of DrawerDeclaration for the styled painting layer.
func (x *DrawerDeclaration) GetClose() gfx.ID { return x.close }

// GetDismiss reads the dismiss of DrawerDeclaration for the styled painting layer.
func (x *DrawerDeclaration) GetDismiss() gfx.ID { return x.dismiss }

// GetHandle reads the handle of DrawerDeclaration for the styled painting layer.
func (x *DrawerDeclaration) GetHandle() gfx.ID { return x.handle }

// GetOffset reads the offset of DrawerDeclaration for the styled painting layer.
func (x *DrawerDeclaration) GetOffset() float32 { return x.offset }

// GetScope reads the scope of DrawerDeclaration for the styled painting layer.
func (x *DrawerDeclaration) GetScope() gfx.ID { return x.scope }

// GetTitle reads the title of DrawerDeclaration for the styled painting layer.
func (x *DrawerDeclaration) GetTitle() string { return x.title }

// GetAt reads the at of DrawerSwipe for the styled painting layer.
func (x *DrawerSwipe) GetAt() float32 { return x.at }

// GetPace reads the pace of DrawerSwipe for the styled painting layer.
func (x *DrawerSwipe) GetPace() float32 { return x.pace }

// GetSeen reads the seen of DrawerSwipe for the styled painting layer.
func (x *DrawerSwipe) GetSeen() uint32 { return x.seen }

// GetWhen reads the when of DrawerSwipe for the styled painting layer.
func (x *DrawerSwipe) GetWhen() time.Duration { return x.when }

// GetAddons reads the addons of Extra for the styled painting layer.
func (x *Extra) GetAddons() int32 { return x.addons }

// GetBind reads the bind of Extra for the styled painting layer.
func (x *Extra) GetBind() int32 { return x.bind }

// GetBorder reads the border of Extra for the styled painting layer.
func (x *Extra) GetBorder() Color { return x.border }

// GetBorderDashed reads the borderDashed of Extra for the styled painting layer.
func (x *Extra) GetBorderDashed() bool { return x.borderDashed }

// GetButtonRecipe reads the buttonRecipe of Extra for the styled painting layer.
func (x *Extra) GetButtonRecipe() func(gfx.Theme, style.Variant, gfx.State) style.ButtonLook {
	return x.buttonRecipe
}

// GetEdges reads the edges of Extra for the styled painting layer.
func (x *Extra) GetEdges() [4]float32 { return x.edges }

// GetFade reads the fade of Extra for the styled painting layer.
func (x *Extra) GetFade() float32 { return x.fade }

// GetFocusGroup reads the focusGroup of Extra for the styled painting layer.
func (x *Extra) GetFocusGroup() FocusAxis { return x.focusGroup }

// GetFocusOrder reads the focusOrder of Extra for the styled painting layer.
func (x *Extra) GetFocusOrder() FocusOrder { return x.focusOrder }

// GetGradient reads the gradient of Extra for the styled painting layer.
func (x *Extra) GetGradient() int32 { return x.gradient }

// GetHit reads the hit of Extra for the styled painting layer.
func (x *Extra) GetHit() gfx.HitOptions { return x.hit }

// GetHitOut reads the hitOut of Extra for the styled painting layer.
func (x *Extra) GetHitOut() [4]float32 { return x.hitOut }

// GetHitExact reads whether a styled control targets its laid-out rectangle for
// the styled painting layer.
func (x *Extra) GetHitExact() bool { return x.hitExact }

// GetIcon reads the icon of Extra for the styled painting layer.
func (x *Extra) GetIcon() gfx.Icon { return x.icon }

// GetIconTransform reads the iconTransform of Extra for the styled painting layer.
func (x *Extra) GetIconTransform() gfx.IconTransform { return x.iconTransform }

// GetLayout reads the layout of Extra for the styled painting layer.
func (x *Extra) GetLayout() Layout { return x.layout }

// GetMaxPct reads the maxPct of Extra for the styled painting layer.
func (x *Extra) GetMaxPct() [2]float32 { return x.maxPct }

// GetMemo reads the memo of Extra for the styled painting layer.
func (x *Extra) GetMemo() int32 { return x.memo }

// GetMoves reads the moves of Extra for the styled painting layer.
func (x *Extra) GetMoves() bool { return x.moves }

// GetNativeView reads the nativeView of Extra for the styled painting layer.
func (x *Extra) GetNativeView() gfx.NativeViewID { return x.nativeView }

// GetPicker reads the picker of Extra for the styled painting layer.
func (x *Extra) GetPicker() *ColorPickerState { return x.picker }

// GetPickerLook reads the pickerLook of Extra for the styled painting layer.
func (x *Extra) GetPickerLook() ColorPickerLook { return x.pickerLook }

// GetRing reads the ring of Extra for the styled painting layer.
func (x *Extra) GetRing() Color { return x.ring }

// GetRingWidth reads the ringWidth of Extra for the styled painting layer.
func (x *Extra) GetRingWidth() float32 { return x.ringWidth }

// GetScrollManual reads the scrollManual of Extra for the styled painting layer.
func (x *Extra) GetScrollManual() bool { return x.scrollManual }

// GetScrollState reads the scrollState of Extra for the styled painting layer.
func (x *Extra) GetScrollState() *ScrollState { return x.scrollState }

// GetShadow reads the shadow of Extra for the styled painting layer.
func (x *Extra) GetShadow() gfx.ShadowLevel { return x.shadow }

// GetSwatch reads the swatch of Extra for the styled painting layer.
func (x *Extra) GetSwatch() bool { return x.swatch }

// GetSwatchChosen reads the swatchChosen of Extra for the styled painting layer.
func (x *Extra) GetSwatchChosen() bool { return x.swatchChosen }

// GetSwatchFlush reads the swatchFlush of Extra for the styled painting layer.
func (x *Extra) GetSwatchFlush() bool { return x.swatchFlush }

// GetSwatchSkipTab reads the swatchSkipTab of Extra for the styled painting layer.
func (x *Extra) GetSwatchSkipTab() bool { return x.swatchSkipTab }

// GetSwatchValue reads the swatchValue of Extra for the styled painting layer.
func (x *Extra) GetSwatchValue() Color { return x.swatchValue }

// GetSwitchRecipe reads the switchRecipe of Extra for the styled painting layer.
func (x *Extra) GetSwitchRecipe() func(gfx.Theme, gfx.State, float32) style.SwitchLook {
	return x.switchRecipe
}

// GetTooltip reads the tooltip of Extra for the styled painting layer.
func (x *Extra) GetTooltip() string { return x.tooltip }

// GetTooltipSide reads the tooltipSide of Extra for the styled painting layer.
func (x *Extra) GetTooltipSide() uint8 { return x.tooltipSide }

// GetWithin reads the within of Extra for the styled painting layer.
func (x *Extra) GetWithin() Rect { return x.within }

// GetEnd reads the end of FieldAddons for the styled painting layer.
func (x *FieldAddons) GetEnd() InputGroupSlot { return x.end }

// GetStart reads the start of FieldAddons for the styled painting layer.
func (x *FieldAddons) GetStart() InputGroupSlot { return x.start }

// GetArea reads the area of Field for the styled painting layer.
func (x *Field) GetArea() *TextareaLayout { return x.area }

// GetBlink reads the blink of Field for the styled painting layer.
func (x *Field) GetBlink() CaretBlink { return x.blink }

// GetChanged reads the changed of Field for the styled painting layer.
func (x *Field) GetChanged() bool { return x.changed }

// GetEditor reads the editor of Field for the styled painting layer.
func (x *Field) GetEditor() *text.Editor { return x.editor }

// GetInput reads the input of Field for the styled painting layer.
func (x *Field) GetInput() window.TextInput { return x.input }

// GetRows reads the rows of Field for the styled painting layer.
func (x *Field) GetRows() bool { return x.rows }

// GetSeen reads the seen of Field for the styled painting layer.
func (x *Field) GetSeen() string { return x.seen }

// GetSpans reads the spans of Field for the styled painting layer.
func (x *Field) GetSpans() []text.Span { return x.spans }

// GetLayout reads the layout of FormFieldState for the styled painting layer.
func (x *FormFieldState) GetLayout() field.Layout { return x.layout }

// GetPaint reads the paint of FormFieldState for the styled painting layer.
func (x *FormFieldState) GetPaint() func(*DrawContext) { return x.paint }

// GetAddons reads the addons of Frame for the styled painting layer.
func (x *Frame) GetAddons() []FieldAddons { return x.addons }

// GetBounds reads the bounds of Frame for the styled painting layer.
func (x *Frame) GetBounds() Rect { return x.bounds }

// GetCaptionFn reads the captionFn of Frame for the styled painting layer.
func (x *Frame) GetCaptionFn() func(f *Frame, n *Node, control Rect) { return x.captionFn }

// GetControls reads the controls of Frame for the styled painting layer.
func (x *Frame) GetControls() any { return x.controls }

// GetDone reads the done of Frame for the styled painting layer.
func (x *Frame) GetDone() bool { return x.done }

// GetDrawContext reads the drawContext of Frame for the styled painting layer.
func (x *Frame) GetDrawContext() DrawContext { return x.drawContext }

// GetGen reads the gen of Frame for the styled painting layer.
func (x *Frame) GetGen() uint32 { return x.gen }

// GetGroupFn reads the groupFn of Frame for the styled painting layer.
func (x *Frame) GetGroupFn() func(f *Frame, i int32, axis FocusAxis, from gfx.Mark) { return x.groupFn }

// GetLayoutCtx reads the layoutCtx of Frame for the styled painting layer.
func (x *Frame) GetLayoutCtx() LayoutContext { return x.layoutCtx }

// GetMemoDepth reads the memoDepth of Frame for the styled painting layer.
func (x *Frame) GetMemoDepth() int { return x.memoDepth }

// GetMemoFn reads the memoFn of Frame for the styled painting layer.
func (x *Frame) GetMemoFn() func(f *Frame, i int32, before bool) bool { return x.memoFn }

// GetNativeMark reads the nativeMark of Frame for the styled painting layer.
func (x *Frame) GetNativeMark() int { return x.nativeMark }

// GetPage reads the page of Frame for the styled painting layer.
func (x *Frame) GetPage() *Page { return x.page }

// GetPaintAddonsFn reads the PaintAddonsFn of Frame for the styled painting layer.
func (x *Frame) GetPaintAddonsFn() func(f *Frame, n *Node, snapshot InputSnapshot, a *FieldAddons) {
	return x.PaintAddonsFn
}

// GetPaintBarsFn reads the paintBarsFn of Frame for the styled painting layer.
func (x *Frame) GetPaintBarsFn() func(f *Frame, n *Node, s *ScrollState) { return x.paintBarsFn }

// GetPaintDrawerFn reads the paintDrawerFn of Frame for the styled painting layer.
func (x *Frame) GetPaintDrawerFn() func(f *Frame, n *Node) { return x.paintDrawerFn }

// GetPaintPickerFn reads the paintPickerFn of Frame for the styled painting layer.
func (x *Frame) GetPaintPickerFn() func(f *Frame, n *Node) { return x.paintPickerFn }

// GetPaintPopoverFn reads the paintPopoverFn of Frame for the styled painting layer.
func (x *Frame) GetPaintPopoverFn() func(f *Frame, i int32, n *Node) { return x.paintPopoverFn }

// GetPaintStops reads the paintStops of Frame for the styled painting layer.
func (x *Frame) GetPaintStops() []gfx.GradientStop { return x.paintStops }

// GetPaintSwatchFn reads the paintSwatchFn of Frame for the styled painting layer.
func (x *Frame) GetPaintSwatchFn() func(f *Frame, n *Node, r Rect) { return x.paintSwatchFn }

// GetPaintTooltipFn reads the paintTooltipFn of Frame for the styled painting layer.
func (x *Frame) GetPaintTooltipFn() func(f *Frame) { return x.paintTooltipFn }

// GetPopupLayer reads the popupLayer of Frame for the styled painting layer.
func (x *Frame) GetPopupLayer() uint16 { return x.popupLayer }

// GetScratch reads the scratch of Frame for the styled painting layer.
func (x *Frame) GetScratch() scratch { return x.scratch }

// GetSettleFn reads the settleFn of Frame for the styled painting layer.
func (x *Frame) GetSettleFn() func(f *Frame, i int32) *bindDecl { return x.settleFn }

// GetShiftedX reads the shiftedX of Frame for the styled painting layer.
func (x *Frame) GetShiftedX() bool { return x.shiftedX }

// GetStyles reads the styles of Frame for the styled painting layer.
func (x *Frame) GetStyles() *Styles { return x.styles }

// GetTooltip reads the tooltip of Frame for the styled painting layer.
func (x *Frame) GetTooltip() any { return x.tooltip }

// GetTooltips reads the tooltips of Frame for the styled painting layer.
func (x *Frame) GetTooltips() bool { return x.tooltips }

// GetWrapCursor reads the wrapCursor of Frame for the styled painting layer.
func (x *Frame) GetWrapCursor() int { return x.wrapCursor }

// GetParts reads the parts of Handlers for the styled painting layer.
func (x Handlers) GetParts() gfx.Scope { return x.parts }

// GetDwell reads the dwell of IconHover for the styled painting layer.
func (x *IconHover) GetDwell() time.Duration { return x.dwell }

// GetSlot reads the slot of IconHover for the styled painting layer.
func (x *IconHover) GetSlot() uint8 { return x.slot }

// GetSpan reads the span of IconHover for the styled painting layer.
func (x *IconHover) GetSpan() time.Duration { return x.span }

// GetState reads the state of IconHover for the styled painting layer.
func (x *IconHover) GetState() *IconHoverState { return x.state }

// GetBeforeX reads the beforeX of LayoutContext for the styled painting layer.
func (x *LayoutContext) GetBeforeX() []float32 { return x.beforeX }

// GetChildren reads the children of LayoutContext for the styled painting layer.
func (x *LayoutContext) GetChildren() []int32 { return x.children }

// GetPhase reads the phase of LayoutContext for the styled painting layer.
func (x *LayoutContext) GetPhase() layoutPhase { return x.phase }

// GetWidth reads the width of LayoutContext for the styled painting layer.
func (x *LayoutContext) GetWidth() float32 { return x.width }

// GetState reads the state of ListRow for the styled painting layer.
func (x *ListRow) GetState() *ListState { return x.state }

// GetCount reads the count of ListState for the styled painting layer.
func (x *ListState) GetCount() int { return x.count }

// GetHeights reads the heights of ListState for the styled painting layer.
func (x *ListState) GetHeights() []float32 { return x.heights }

// GetList reads the list of ListState for the styled painting layer.
func (x *ListState) GetList() listInternals { return x.list }

// GetTotal reads the total of ListState for the styled painting layer.
func (x *ListState) GetTotal() float64 { return x.total }

// GetTree reads the tree of ListState for the styled painting layer.
func (x *ListState) GetTree() listTree { return x.tree }

// GetLookBackground reads the lookBackground of Look for the styled painting layer.
func (x *Look) GetLookBackground() Color { return x.lookBackground }

// GetLookBorder reads the lookBorder of Look for the styled painting layer.
func (x *Look) GetLookBorder() Color { return x.lookBorder }

// GetLookColor reads the lookColor of Look for the styled painting layer.
func (x *Look) GetLookColor() Color { return x.lookColor }

// GetLookDashed reads the lookDashed of Look for the styled painting layer.
func (x *Look) GetLookDashed() bool { return x.lookDashed }

// GetLookOpacity reads the lookOpacity of Look for the styled painting layer.
func (x *Look) GetLookOpacity() float32 { return x.lookOpacity }

// GetLookRing reads the lookRing of Look for the styled painting layer.
func (x *Look) GetLookRing() Color { return x.lookRing }

// GetLookRingWidth reads the lookRingWidth of Look for the styled painting layer.
func (x *Look) GetLookRingWidth() float32 { return x.lookRingWidth }

// GetLookShadow reads the lookShadow of Look for the styled painting layer.
func (x *Look) GetLookShadow() gfx.ShadowLevel { return x.lookShadow }

// GetSet reads the set of Look for the styled painting layer.
func (x *Look) GetSet() lookFields { return x.set }

// GetI reads the i of MenuItem for the styled painting layer.
func (x *MenuItem) GetI() int { return x.i }

// GetMenu reads the menu of MenuItem for the styled painting layer.
func (x *MenuItem) GetMenu() *MenuItems { return x.menu }

// GetAt reads the at of MenuItems for the styled painting layer.
func (x *MenuItems) GetAt() Rect { return x.at }

// GetContext reads the context of MenuItems for the styled painting layer.
func (x *MenuItems) GetContext() bool { return x.context }

// GetDrawItem reads the drawItem of MenuItems for the styled painting layer.
func (x *MenuItems) GetDrawItem() func(MenuItemParts) { return x.drawItem }

// GetDrawPanel reads the drawPanel of MenuItems for the styled painting layer.
func (x *MenuItems) GetDrawPanel() func(*gfx.Canvas, gfx.Rect, gfx.Theme) { return x.drawPanel }

// GetF reads the f of MenuItems for the styled painting layer.
func (x *MenuItems) GetF() *Frame { return x.f }

// GetFocusFirst reads the focusFirst of MenuItems for the styled painting layer.
func (x *MenuItems) GetFocusFirst() bool { return x.focusFirst }

// GetFocusSub reads the focusSub of MenuItems for the styled painting layer.
func (x *MenuItems) GetFocusSub() bool { return x.focusSub }

// GetIntent reads the intent of MenuItems for the styled painting layer.
func (x *MenuItems) GetIntent() HoverIntent { return x.intent }

// GetItemPainter reads the itemPainter of MenuItems for the styled painting layer.
func (x *MenuItems) GetItemPainter() func(style.MenuItemPaint) { return x.itemPainter }

// GetKeys reads the keys of MenuItems for the styled painting layer.
func (x *MenuItems) GetKeys() [6]string { return x.keys }

// GetLayer reads the layer of MenuItems for the styled painting layer.
func (x *MenuItems) GetLayer() uint16 { return x.layer }

// GetOpen reads the open of MenuItems for the styled painting layer.
func (x *MenuItems) GetOpen() *bool { return x.open }

// GetPage reads the page of MenuItems for the styled painting layer.
func (x *MenuItems) GetPage() *Page { return x.page }

// GetPaint reads the paint of MenuItems for the styled painting layer.
func (x *MenuItems) GetPaint() func(*DrawContext) { return x.paint }

// GetPanelPainter reads the panelPainter of MenuItems for the styled painting layer.
func (x *MenuItems) GetPanelPainter() func(*gfx.Canvas, gfx.Rect, gfx.Theme) { return x.panelPainter }

// GetRoot reads the root of MenuItems for the styled painting layer.
func (x *MenuItems) GetRoot() *MenuItems { return x.root }

// GetRowSize reads the rowSize of MenuItems for the styled painting layer.
func (x *MenuItems) GetRowSize() float32 { return x.rowSize }

// GetScope reads the scope of MenuItems for the styled painting layer.
func (x *MenuItems) GetScope() gfx.Scope { return x.scope }

// GetSeen reads the seen of MenuItems for the styled painting layer.
func (x *MenuItems) GetSeen() uint32 { return x.seen }

// GetShown reads the shown of MenuItems for the styled painting layer.
func (x *MenuItems) GetShown() bool { return x.shown }

// GetSide reads the side of MenuItems for the styled painting layer.
func (x *MenuItems) GetSide() menuSide { return x.side }

// GetSub reads the sub of MenuItems for the styled painting layer.
func (x *MenuItems) GetSub() int { return x.sub }

// GetSubMenu reads the subMenu of MenuItems for the styled painting layer.
func (x *MenuItems) GetSubMenu() *MenuItems { return x.subMenu }

// GetTypeahead reads the typeahead of MenuItems for the styled painting layer.
func (x *MenuItems) GetTypeahead() MenuTypeahead { return x.typeahead }

// GetUnpainted reads the unpainted of MenuItems for the styled painting layer.
func (x *MenuItems) GetUnpainted() bool { return x.unpainted }

// GetWidthMax reads the widthMax of MenuItems for the styled painting layer.
func (x *MenuItems) GetWidthMax() float32 { return x.widthMax }

// GetWidthMin reads the widthMin of MenuItems for the styled painting layer.
func (x *MenuItems) GetWidthMin() float32 { return x.widthMin }

// GetChar reads the char of MenuRow for the styled painting layer.
func (x *MenuRow) GetChar() byte { return x.char }

// GetChecked reads the checked of MenuRow for the styled painting layer.
func (x *MenuRow) GetChecked() int8 { return x.checked }

// GetDescription reads the description of MenuRow for the styled painting layer.
func (x *MenuRow) GetDescription() string { return x.description }

// GetDestructive reads the destructive of MenuRow for the styled painting layer.
func (x *MenuRow) GetDestructive() bool { return x.destructive }

// GetDisabled reads the disabled of MenuRow for the styled painting layer.
func (x *MenuRow) GetDisabled() bool { return x.disabled }

// GetHeight reads the height of MenuRow for the styled painting layer.
func (x *MenuRow) GetHeight() float32 { return x.height }

// GetIcon reads the icon of MenuRow for the styled painting layer.
func (x *MenuRow) GetIcon() gfx.Icon { return x.icon }

// GetKey reads the key of MenuRow for the styled painting layer.
func (x *MenuRow) GetKey() gfx.Key { return x.key }

// GetModifiers reads the modifiers of MenuRow for the styled painting layer.
func (x *MenuRow) GetModifiers() gfx.Modifiers { return x.modifiers }

// GetPageContent reads the pageContent of MenuRow for the styled painting layer.
func (x *MenuRow) GetPageContent() func(c C, bounds Rect) { return x.pageContent }

// GetSubmenu reads the submenu of MenuRow for the styled painting layer.
func (x *MenuRow) GetSubmenu() bool { return x.submenu }

// GetTrailing reads the trailing of MenuRow for the styled painting layer.
func (x *MenuRow) GetTrailing() func(Surface *gfx.Canvas, bounds gfx.Rect, Palette gfx.Theme) {
	return x.trailing
}

// GetTrailingWidth reads the trailingWidth of MenuRow for the styled painting layer.
func (x *MenuRow) GetTrailingWidth() float32 { return x.trailingWidth }

// GetApplication reads the application of Page for the styled painting layer.
func (x *Page) GetApplication() any { return x.application }

// GetAreaKey reads the areaKey of Page for the styled painting layer.
func (x *Page) GetAreaKey() func(*Field, gfx.KeyEvent) gfx.EventResult { return x.areaKey }

// GetBaseKey reads the baseKey of Page for the styled painting layer.
func (x *Page) GetBaseKey() func(gfx.KeyEvent) gfx.EventResult { return x.baseKey }

// GetBasePointer reads the basePointer of Page for the styled painting layer.
func (x *Page) GetBasePointer() func(gfx.PointerEvent) gfx.EventResult { return x.basePointer }

// GetBaseScroll reads the baseScroll of Page for the styled painting layer.
func (x *Page) GetBaseScroll() func(gfx.ScrollEvent) gfx.EventResult { return x.baseScroll }

// GetBaseShortcut reads the baseShortcut of Page for the styled painting layer.
func (x *Page) GetBaseShortcut() func(gfx.KeyEvent) gfx.EventResult { return x.baseShortcut }

// GetBaseText reads the baseText of Page for the styled painting layer.
func (x *Page) GetBaseText() window.TextInputProvider { return x.baseText }

// GetCheckMemos reads the checkMemos of Page for the styled painting layer.
func (x *Page) GetCheckMemos() bool { return x.checkMemos }

// GetComboRoute reads the comboRoute of Page for the styled painting layer.
func (x *Page) GetComboRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) { return x.comboRoute }

// GetCombos reads the combos of Page for the styled painting layer.
func (x *Page) GetCombos() []*ComboState { return x.combos }

// GetConfig reads the config of Page for the styled painting layer.
func (x *Page) GetConfig() PageConfig { return x.config }

// GetContexts reads the contexts of Page for the styled painting layer.
func (x *Page) GetContexts() []contextTarget { return x.contexts }

// GetControls reads the controls of Page for the styled painting layer.
func (x *Page) GetControls() []controlEntry { return x.controls }

// GetCtx reads the ctx of Page for the styled painting layer.
func (x *Page) GetCtx() Ctx { return x.ctx }

// GetDrag reads the drag of Page for the styled painting layer.
func (x *Page) GetDrag() *DragState { return x.drag }

// GetDrags reads the drags of Page for the styled painting layer.
func (x *Page) GetDrags() []*DragState { return x.drags }

// GetEffectCancel reads the effectCancel of Page for the styled painting layer.
func (x *Page) GetEffectCancel() context.CancelFunc { return x.effectCancel }

// GetEffectContext reads the effectContext of Page for the styled painting layer.
func (x *Page) GetEffectContext() context.Context { return x.effectContext }

// GetEffects reads the effects of Page for the styled painting layer.
func (x *Page) GetEffects() []effectState { return x.effects }

// GetEpoch reads the epoch of Page for the styled painting layer.
func (x *Page) GetEpoch() uint32 { return x.epoch }

// GetFieldKeyRoute reads the fieldKeyRoute of Page for the styled painting layer.
func (x *Page) GetFieldKeyRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) {
	return x.fieldKeyRoute
}

// GetFields reads the fields of Page for the styled painting layer.
func (x *Page) GetFields() []fieldEntry { return x.fields }

// GetFocusFields reads the focusFields of Page for the styled painting layer.
func (x *Page) GetFocusFields() map[any]any { return x.focusFields }

// GetFocusPaint reads the focusPaint of Page for the styled painting layer.
func (x *Page) GetFocusPaint() []focusState { return x.focusPaint }

// GetFresh reads the fresh of Page for the styled painting layer.
func (x *Page) GetFresh() slotPool { return x.fresh }

// GetFreshAt reads the freshAt of Page for the styled painting layer.
func (x *Page) GetFreshAt() gfx.Scope { return x.freshAt }

// GetGroupIDs reads the groupIDs of Page for the styled painting layer.
func (x *Page) GetGroupIDs() []gfx.ID { return x.groupIDs }

// GetGroupRoute reads the groupRoute of Page for the styled painting layer.
func (x *Page) GetGroupRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) { return x.groupRoute }

// GetGroupStops reads the groupStops of Page for the styled painting layer.
func (x *Page) GetGroupStops() map[gfx.ID]groupStop { return x.groupStops }

// GetGroups reads the groups of Page for the styled painting layer.
func (x *Page) GetGroups() []focusGroup { return x.groups }

// GetHeld reads the held of Page for the styled painting layer.
func (x *Page) GetHeld() [2]armedButton { return x.held }

// GetHoverArea reads the hoverArea of Page for the styled painting layer.
func (x *Page) GetHoverArea() gfx.ID { return x.hoverArea }

// GetIntKeys reads the intKeys of Page for the styled painting layer.
func (x *Page) GetIntKeys() map[intKey]keyEntry { return x.intKeys }

// GetInvalidator reads the invalidator of Page for the styled painting layer.
func (x *Page) GetInvalidator() window.Invalidator { return x.invalidator }

// GetKeyName reads the keyName of Page for the styled painting layer.
func (x *Page) GetKeyName() gfx.Name { return x.keyName }

// GetKeyRoute reads the keyRoute of Page for the styled painting layer.
func (x *Page) GetKeyRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) { return x.keyRoute }

// GetKeys reads the keys of Page for the styled painting layer.
func (x *Page) GetKeys() map[keyKey]keyEntry { return x.keys }

// GetKindName reads the kindName of Page for the styled painting layer.
func (x *Page) GetKindName() [pkKinds]gfx.Name { return x.kindName }

// GetLabels reads the labels of Page for the styled painting layer.
func (x *Page) GetLabels() []string { return x.labels }

// GetListRoute reads the listRoute of Page for the styled painting layer.
func (x *Page) GetListRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) { return x.listRoute }

// GetListRows reads the listRows of Page for the styled painting layer.
func (x *Page) GetListRows() []int32 { return x.listRows }

// GetListStates reads the listStates of Page for the styled painting layer.
func (x *Page) GetListStates() []*ListState { return x.listStates }

// GetLists reads the lists of Page for the styled painting layer.
func (x *Page) GetLists() []listDecl { return x.lists }

// GetMemoBoxes reads the memoBoxes of Page for the styled painting layer.
func (x *Page) GetMemoBoxes() []memoBox { return x.memoBoxes }

// GetMemoChecks reads the memoChecks of Page for the styled painting layer.
func (x *Page) GetMemoChecks() []memoCheckOf { return x.memoChecks }

// GetMemos reads the memos of Page for the styled painting layer.
func (x *Page) GetMemos() map[gfx.ID]*memoEntry { return x.memos }

// GetMenuName reads the menuName of Page for the styled painting layer.
func (x *Page) GetMenuName() gfx.Name { return x.menuName }

// GetMenuRoute reads the menuRoute of Page for the styled painting layer.
func (x *Page) GetMenuRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) { return x.menuRoute }

// GetMenus reads the menus of Page for the styled painting layer.
func (x *Page) GetMenus() []*MenuItems { return x.menus }

// GetNames reads the names of Page for the styled painting layer.
func (x *Page) GetNames() map[gfx.ID]nameUse { return x.names }

// GetOverlay reads the overlay of Page for the styled painting layer.
func (x *Page) GetOverlay() bool { return x.overlay }

// GetPointer reads the pointer of Page for the styled painting layer.
func (x *Page) GetPointer() gfx.Point { return x.pointer }

// GetPointerIn reads the pointerIn of Page for the styled painting layer.
func (x *Page) GetPointerIn() bool { return x.pointerIn }

// GetPointerRoute reads the pointerRoute of Page for the styled painting layer.
func (x *Page) GetPointerRoute() func(*Page, gfx.PointerEvent) (gfx.EventResult, bool) {
	return x.pointerRoute
}

// GetRecording reads the recording of Page for the styled painting layer.
func (x *Page) GetRecording() []*memoEntry { return x.recording }

// GetRetry reads the retry of Page for the styled painting layer.
func (x *Page) GetRetry() bool { return x.retry }

// GetRoot reads the root of Page for the styled painting layer.
func (x *Page) GetRoot() gfx.Scope { return x.root }

// GetRoveRoute reads the roveRoute of Page for the styled painting layer.
func (x *Page) GetRoveRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) { return x.roveRoute }

// GetRovers reads the rovers of Page for the styled painting layer.
func (x *Page) GetRovers() []rover { return x.rovers }

// GetScopes reads the scopes of Page for the styled painting layer.
func (x *Page) GetScopes() []*protoScope { return x.scopes }

// GetSelectRoute reads the selectRoute of Page for the styled painting layer.
func (x *Page) GetSelectRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) {
	return x.selectRoute
}

// GetSelects reads the selects of Page for the styled painting layer.
func (x *Page) GetSelects() []*SelectState { return x.selects }

// GetShortcutRoute reads the shortcutRoute of Page for the styled painting layer.
func (x *Page) GetShortcutRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) {
	return x.shortcutRoute
}

// GetShortcuts reads the shortcuts of Page for the styled painting layer.
func (x *Page) GetShortcuts() []shortcutEntry { return x.shortcuts }

// GetSliderRoute reads the sliderRoute of Page for the styled painting layer.
func (x *Page) GetSliderRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) {
	return x.sliderRoute
}

// GetSliders reads the sliders of Page for the styled painting layer.
func (x *Page) GetSliders() []*SliderState { return x.sliders }

// GetStale reads the stale of Page for the styled painting layer.
func (x *Page) GetStale() *atomic.Bool { return &x.stale }

// GetTables reads the tables of Page for the styled painting layer.
func (x *Page) GetTables() []tableRoute { return x.tables }

// GetTextRoute reads the textRoute of Page for the styled painting layer.
func (x *Page) GetTextRoute() func(*Page, gfx.ID) *window.TextInput { return x.textRoute }

// GetTexts reads the texts of Page for the styled painting layer.
func (x *Page) GetTexts() map[gfx.ID]*boundText { return x.texts }

// GetThemeEpoch reads the themeEpoch of Page for the styled painting layer.
func (x *Page) GetThemeEpoch() uint32 { return x.themeEpoch }

// GetThemeSet reads the themeSet of Page for the styled painting layer.
func (x *Page) GetThemeSet() bool { return x.themeSet }

// GetThemed reads the themed of Page for the styled painting layer.
func (x *Page) GetThemed() bool { return x.themed }

// GetTree reads the tree of Page for the styled painting layer.
func (x *Page) GetTree() *gfx.Tree { return x.tree }

// GetTreeRoute reads the treeRoute of Page for the styled painting layer.
func (x *Page) GetTreeRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) { return x.treeRoute }

// GetTrees reads the trees of Page for the styled painting layer.
func (x *Page) GetTrees() []*TreeState { return x.trees }

// GetTypeRoute reads the typeRoute of Page for the styled painting layer.
func (x *Page) GetTypeRoute() func(*Page, gfx.KeyEvent) (gfx.EventResult, bool) { return x.typeRoute }

// GetWheelRoute reads the wheelRoute of Page for the styled painting layer.
func (x *Page) GetWheelRoute() func(*Page, gfx.ScrollEvent) (gfx.EventResult, bool) {
	return x.wheelRoute
}

// GetWheelXRoute reads the wheelXRoute of Page for the styled painting layer.
func (x *Page) GetWheelXRoute() func(*Page, gfx.ScrollEvent) (moved, only bool) { return x.wheelXRoute }

// GetWindow reads the window of Page for the styled painting layer.
func (x *Page) GetWindow() bool { return x.window }

// GetCount reads the count of PaginationDecl for the styled painting layer.
func (x *PaginationDecl) GetCount() int32 { return x.count }

// GetStart reads the start of PaginationDecl for the styled painting layer.
func (x *PaginationDecl) GetStart() int32 { return x.start }

// GetScope reads the scope of Place for the styled painting layer.
func (x *Place) GetScope() gfx.Scope { return x.scope }

// GetLayer reads the layer of PopoverDecl for the styled painting layer.
func (x *PopoverDecl) GetLayer() uint16 { return x.layer }

// GetSpec reads the spec of PopoverDecl for the styled painting layer.
func (x *PopoverDecl) GetSpec() PopoverSpec { return x.spec }

// GetBarLit reads the barLit of ScrollState for the styled painting layer.
func (x *ScrollState) GetBarLit() bool { return x.barLit }

// GetBarOffset reads the barOffset of ScrollState for the styled painting layer.
func (x *ScrollState) GetBarOffset() float32 { return x.barOffset }

// GetBarOffsetX reads the barOffsetX of ScrollState for the styled painting layer.
func (x *ScrollState) GetBarOffsetX() float32 { return x.barOffsetX }

// GetBarSeen reads the barSeen of ScrollState for the styled painting layer.
func (x *ScrollState) GetBarSeen() bool { return x.barSeen }

// GetBarSince reads the barSince of ScrollState for the styled painting layer.
func (x *ScrollState) GetBarSince() time.Duration { return x.barSince }

// GetBarSize reads the barSize of ScrollState for the styled painting layer.
func (x *ScrollState) GetBarSize() float32 { return x.barSize }

// GetBars reads the bars of ScrollState for the styled painting layer.
func (x *ScrollState) GetBars() bool { return x.bars }

// GetBarsAlways reads the barsAlways of ScrollState for the styled painting layer.
func (x *ScrollState) GetBarsAlways() bool { return x.barsAlways }

// GetDetached reads the detached of ScrollState for the styled painting layer.
func (x *ScrollState) GetDetached() bool { return x.detached }

// GetDragging reads the dragging of ScrollState for the styled painting layer.
func (x *ScrollState) GetDragging() bool { return x.dragging }

// GetFollow reads the follow of ScrollState for the styled painting layer.
func (x *ScrollState) GetFollow() bool { return x.follow }

// GetGrab reads the grab of ScrollState for the styled painting layer.
func (x *ScrollState) GetGrab() float32 { return x.grab }

// GetHorizontalID reads the horizontalID of ScrollState for the styled painting layer.
func (x *ScrollState) GetHorizontalID() gfx.ID { return x.horizontalID }

// GetHorizontalLabel reads the horizontalLabel of ScrollState for the styled painting layer.
func (x *ScrollState) GetHorizontalLabel() string { return x.horizontalLabel }

// GetSideways reads the sideways of ScrollState for the styled painting layer.
func (x *ScrollState) GetSideways() bool { return x.sideways }

// GetToEnd reads the toEnd of ScrollState for the styled painting layer.
func (x *ScrollState) GetToEnd() bool { return x.toEnd }

// GetVerticalID reads the verticalID of ScrollState for the styled painting layer.
func (x *ScrollState) GetVerticalID() gfx.ID { return x.verticalID }

// GetVerticalLabel reads the verticalLabel of ScrollState for the styled painting layer.
func (x *ScrollState) GetVerticalLabel() string { return x.verticalLabel }

// GetAsked reads the asked of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetAsked() bool { return x.asked }

// GetDisabled reads the disabled of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetDisabled() []bool { return x.disabled }

// GetHint reads the hint of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetHint() string { return x.hint }

// GetIcons reads the icons of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetIcons() []gfx.Icon { return x.icons }

// GetLabels reads the labels of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetLabels() []string { return x.labels }

// GetLayer reads the layer of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetLayer() uint16 { return x.layer }

// GetPick reads the pick of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetPick() int { return x.pick }

// GetPicked reads the picked of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetPicked() bool { return x.picked }

// GetPopup reads the popup of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetPopup() int32 { return x.popup }

// GetRects reads the rects of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetRects() int32 { return x.rects }

// GetScope reads the scope of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetScope() gfx.Scope { return x.scope }

// GetSelected reads the selected of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetSelected() int { return x.selected }

// GetState reads the state of SelectDecl for the styled painting layer.
func (x *SelectDecl) GetState() *SelectState { return x.state }

// GetChosen reads the chosen of SelectState for the styled painting layer.
func (x *SelectState) GetChosen() int { return x.chosen }

// GetDisabled reads the disabled of SelectState for the styled painting layer.
func (x *SelectState) GetDisabled() []bool { return x.disabled }

// GetLabels reads the labels of SelectState for the styled painting layer.
func (x *SelectState) GetLabels() []string { return x.labels }

// GetListMaxHeight reads the listMaxHeight of SelectState for the styled painting layer.
func (x *SelectState) GetListMaxHeight() float32 { return x.listMaxHeight }

// GetListMaxWidth reads the listMaxWidth of SelectState for the styled painting layer.
func (x *SelectState) GetListMaxWidth() float32 { return x.listMaxWidth }

// GetOptionsMeasured reads the optionsMeasured of SelectState for the styled painting layer.
func (x *SelectState) GetOptionsMeasured() bool { return x.optionsMeasured }

// GetOptionsSize reads the optionsSize of SelectState for the styled painting layer.
func (x *SelectState) GetOptionsSize() float32 { return x.optionsSize }

// GetOptionsWidest reads the optionsWidest of SelectState for the styled painting layer.
func (x *SelectState) GetOptionsWidest() float32 { return x.optionsWidest }

// GetScope reads the scope of SelectState for the styled painting layer.
func (x *SelectState) GetScope() gfx.Scope { return x.scope }

// GetSelected reads the selected of SelectState for the styled painting layer.
func (x *SelectState) GetSelected() int { return x.selected }

// GetTrigger reads the trigger of SelectState for the styled painting layer.
func (x *SelectState) GetTrigger() gfx.ID { return x.trigger }

// GetTyped reads the typed of SelectState for the styled painting layer.
func (x *SelectState) GetTyped() [16]byte { return x.typed }

// GetTypedAt reads the typedAt of SelectState for the styled painting layer.
func (x *SelectState) GetTypedAt() time.Time { return x.typedAt }

// GetTypedLen reads the typedLen of SelectState for the styled painting layer.
func (x *SelectState) GetTypedLen() int { return x.typedLen }

// GetWheel reads the wheel of SelectState for the styled painting layer.
func (x *SelectState) GetWheel() float32 { return x.wheel }

// GetChanged reads the changed of SliderDecl for the styled painting layer.
func (x *SliderDecl) GetChanged() bool { return x.changed }

// GetSettled reads the settled of SliderDecl for the styled painting layer.
func (x *SliderDecl) GetSettled() bool { return x.settled }

// GetState reads the state of SliderDecl for the styled painting layer.
func (x *SliderDecl) GetState() *SliderState { return x.state }

// GetArea reads the area of SliderState for the styled painting layer.
func (x *SliderState) GetArea() Rect { return x.area }

// GetDragging reads the dragging of SliderState for the styled painting layer.
func (x *SliderState) GetDragging() bool { return x.dragging }

// GetGrab reads the grab of SliderState for the styled painting layer.
func (x *SliderState) GetGrab() float32 { return x.grab }

// GetMax reads the max of SliderState for the styled painting layer.
func (x *SliderState) GetMax() float32 { return x.max }

// GetMin reads the min of SliderState for the styled painting layer.
func (x *SliderState) GetMin() float32 { return x.min }

// GetMoved reads the moved of SliderState for the styled painting layer.
func (x *SliderState) GetMoved() bool { return x.moved }

// GetOff reads the off of SliderState for the styled painting layer.
func (x *SliderState) GetOff() bool { return x.off }

// GetStep reads the step of SliderState for the styled painting layer.
func (x *SliderState) GetStep() float32 { return x.step }

// GetTextOf reads the textOf of SliderState for the styled painting layer.
func (x *SliderState) GetTextOf() float32 { return x.textOf }

// GetHint reads the hint of SplitButtonDecl for the styled painting layer.
func (x *SplitButtonDecl) GetHint() string { return x.hint }

// GetIcon reads the icon of SplitButtonDecl for the styled painting layer.
func (x *SplitButtonDecl) GetIcon() gfx.Icon { return x.icon }

// GetMenu reads the menu of SplitButtonDecl for the styled painting layer.
func (x *SplitButtonDecl) GetMenu() gfx.ID { return x.menu }

// GetMenuHint reads the menuHint of SplitButtonDecl for the styled painting layer.
func (x *SplitButtonDecl) GetMenuHint() string { return x.menuHint }

// GetMenuLabel reads the menuLabel of SplitButtonDecl for the styled painting layer.
func (x *SplitButtonDecl) GetMenuLabel() string { return x.menuLabel }

// GetOpen reads the open of SplitButtonDecl for the styled painting layer.
func (x *SplitButtonDecl) GetOpen() bool { return x.open }

// GetPrimary reads the primary of SplitButtonDecl for the styled painting layer.
func (x *SplitButtonDecl) GetPrimary() gfx.ID { return x.primary }

// GetTriggerWidth reads the triggerWidth of SplitButtonDecl for the styled painting layer.
func (x *SplitButtonDecl) GetTriggerWidth() float32 { return x.triggerWidth }

// GetMinFirst reads the minFirst of SplitDecl for the styled painting layer.
func (x *SplitDecl) GetMinFirst() float32 { return x.minFirst }

// GetMinSecond reads the minSecond of SplitDecl for the styled painting layer.
func (x *SplitDecl) GetMinSecond() float32 { return x.minSecond }

// GetState reads the state of SplitDecl for the styled painting layer.
func (x *SplitDecl) GetState() *SplitterState { return x.state }

// GetThickness reads the thickness of SplitDecl for the styled painting layer.
func (x *SplitDecl) GetThickness() float32 { return x.thickness }

// GetDragging reads the dragging of SplitterState for the styled painting layer.
func (x *SplitterState) GetDragging() bool { return x.dragging }

// GetHi reads the hi of SplitterState for the styled painting layer.
func (x *SplitterState) GetHi() float32 { return x.hi }

// GetLo reads the lo of SplitterState for the styled painting layer.
func (x *SplitterState) GetLo() float32 { return x.lo }

// GetStartPointer reads the startPointer of SplitterState for the styled painting layer.
func (x *SplitterState) GetStartPointer() float32 { return x.startPointer }

// GetStartPos reads the startPos of SplitterState for the styled painting layer.
func (x *SplitterState) GetStartPos() float32 { return x.startPos }

// GetThickness reads the thickness of SplitterState for the styled painting layer.
func (x *SplitterState) GetThickness() float32 { return x.thickness }

// GetRows reads the rows of TableDecl for the styled painting layer.
func (x *TableDecl) GetRows() int { return x.rows }

// GetSpec reads the spec of TableDecl for the styled painting layer.
func (x *TableDecl) GetSpec() TableSpec { return x.spec }

// GetState reads the state of TableDecl for the styled painting layer.
func (x *TableDecl) GetState() *TableState { return x.state }

// GetCellLefts reads the cellLefts of TableState for the styled painting layer.
func (x *TableState) GetCellLefts() []float32 { return x.cellLefts }

// GetCellRowHeight reads the cellRowHeight of TableState for the styled painting layer.
func (x *TableState) GetCellRowHeight() float32 { return x.cellRowHeight }

// GetCellTop reads the cellTop of TableState for the styled painting layer.
func (x *TableState) GetCellTop() float32 { return x.cellTop }

// GetDescending reads the descending of TableState for the styled painting layer.
func (x *TableState) GetDescending() bool { return x.descending }

// GetPage reads the page of TableState for the styled painting layer.
func (x *TableState) GetPage() int32 { return x.page }

// GetReveal reads the reveal of TableState for the styled painting layer.
func (x *TableState) GetReveal() bool { return x.reveal }

// GetRowEnd reads the rowEnd of TableState for the styled painting layer.
func (x *TableState) GetRowEnd() int32 { return x.rowEnd }

// GetRowFirst reads the rowFirst of TableState for the styled painting layer.
func (x *TableState) GetRowFirst() int32 { return x.rowFirst }

// GetRows reads the rows of TableState for the styled painting layer.
func (x *TableState) GetRows() int32 { return x.rows }

// GetScope reads the scope of TableState for the styled painting layer.
func (x *TableState) GetScope() gfx.Scope { return x.scope }

// GetSelected reads the selected of TableState for the styled painting layer.
func (x *TableState) GetSelected() int32 { return x.selected }

// GetSortColumn reads the sortColumn of TableState for the styled painting layer.
func (x *TableState) GetSortColumn() int32 { return x.sortColumn }

// GetLabels reads the labels of TabsDecl for the styled painting layer.
func (x *TabsDecl) GetLabels() []string { return x.labels }

// GetIcon reads the icon function of TabsDecl for the styled painting layer.
func (x *TabsDecl) GetIcon() func(index int) gfx.Icon { return x.icon }

// GetLine reads the line of TabsDecl for the styled painting layer.
func (x *TabsDecl) GetLine() bool { return x.line }

// GetScope reads the scope of TabsDecl for the styled painting layer.
func (x *TabsDecl) GetScope() gfx.Scope { return x.scope }

// GetSelected reads the selected of TabsDecl for the styled painting layer.
func (x *TabsDecl) GetSelected() int { return x.selected }

// GetVertical reads the vertical of TabsDecl for the styled painting layer.
func (x *TabsDecl) GetVertical() bool { return x.vertical }

// GetActed reads the acted of Toaster for the styled painting layer.
func (x *Toaster) GetActed() ToastID { return x.acted }

// GetEntries reads the entries of Toaster for the styled painting layer.
func (x *Toaster) GetEntries() []ToastQueueEntry { return x.entries }

// GetQueue reads the queue of Toaster for the styled painting layer.
func (x *Toaster) GetQueue() ToastQueue { return x.queue }

// GetScope reads the scope of Toaster for the styled painting layer.
func (x *Toaster) GetScope() gfx.Scope { return x.scope }

// GetStack reads the stack of Toaster for the styled painting layer.
func (x *Toaster) GetStack() []SonnerToast { return x.stack }

// GetToasts reads the toasts of Toaster for the styled painting layer.
func (x *Toaster) GetToasts() []toast { return x.toasts }

// GetGen reads the gen of TreeRow for the styled painting layer.
func (x *TreeRow) GetGen() uint32 { return x.gen }

// GetIndex reads the index of TreeRow for the styled painting layer.
func (x *TreeRow) GetIndex() int { return x.index }

// GetItem reads the item of TreeRow for the styled painting layer.
func (x *TreeRow) GetItem() style.TreeItem { return x.item }

// GetKey reads the key of TreeRow for the styled painting layer.
func (x *TreeRow) GetKey() int { return x.key }

// GetC reads the c of TreeState for the styled painting layer.
func (x *TreeState) GetC() C { return x.c }

// GetChevrons reads the chevrons of TreeState for the styled painting layer.
func (x *TreeState) GetChevrons() gfx.Scope { return x.chevrons }

// GetChosen reads the chosen of TreeState for the styled painting layer.
func (x *TreeState) GetChosen() int { return x.chosen }

// GetDeclare reads the declare of TreeState for the styled painting layer.
func (x *TreeState) GetDeclare() func(i int, Box Box) { return x.declare }

// GetDeclared reads the declared of TreeState for the styled painting layer.
func (x *TreeState) GetDeclared() []TreeRow { return x.declared }

// GetDrag reads the drag of TreeState for the styled painting layer.
func (x *TreeState) GetDrag() treeDrag { return x.drag }

// GetDropped reads the dropped of TreeState for the styled painting layer.
func (x *TreeState) GetDropped() TreeMove { return x.dropped }

// GetF reads the f of TreeState for the styled painting layer.
func (x *TreeState) GetF() *Frame { return x.f }

// GetN reads the n of TreeState for the styled painting layer.
func (x *TreeState) GetN() int { return x.n }

// GetNameOf reads the nameOf of TreeState for the styled painting layer.
func (x *TreeState) GetNameOf() func(int) string { return x.nameOf }

// GetNames reads the names of TreeState for the styled painting layer.
func (x *TreeState) GetNames() []MenuTypeaheadItem { return x.names }

// GetPaint reads the paint of TreeState for the styled painting layer.
func (x *TreeState) GetPaint() func(*DrawContext) { return x.paint }

// GetRowLabel reads the rowLabel of TreeState for the styled painting layer.
func (x *TreeState) GetRowLabel() func(int) string { return x.rowLabel }

// GetRows reads the rows of TreeState for the styled painting layer.
func (x *TreeState) GetRows() gfx.Scope { return x.rows }

// GetScope reads the scope of TreeState for the styled painting layer.
func (x *TreeState) GetScope() gfx.Scope { return x.scope }

// GetToggled reads the toggled of TreeState for the styled painting layer.
func (x *TreeState) GetToggled() int { return x.toggled }

// GetTypeahead reads the typeahead of TreeState for the styled painting layer.
func (x *TreeState) GetTypeahead() MenuTypeahead { return x.typeahead }

// GetAlertDialogs reads the alertDialogs of Widgets for the styled painting layer.
func (x *Widgets) GetAlertDialogs() []AlertDialogDecl { return x.alertDialogs }

// GetCells reads the cells of Widgets for the styled painting layer.
func (x *Widgets) GetCells() []string { return x.cells }

// GetChoices reads the choices of Widgets for the styled painting layer.
func (x *Widgets) GetChoices() []style.SegmentOption { return x.choices }

// GetDialogs reads the dialogs of Widgets for the styled painting layer.
func (x *Widgets) GetDialogs() []DialogDecl { return x.dialogs }

// GetDrawers reads the drawers of Widgets for the styled painting layer.
func (x *Widgets) GetDrawers() []drawerState { return x.drawers }

// GetGroups reads the groups of Widgets for the styled painting layer.
func (x *Widgets) GetGroups() []InputGroupSnapshot { return x.groups }

// GetInputs reads the inputs of Widgets for the styled painting layer.
func (x *Widgets) GetInputs() []InputSnapshot { return x.inputs }

// GetLayer reads the layer of Widgets for the styled painting layer.
func (x *Widgets) GetLayer() uint16 { return x.layer }

// GetPaginations reads the paginations of Widgets for the styled painting layer.
func (x *Widgets) GetPaginations() []PaginationDecl { return x.paginations }

// GetPopoverFocus reads the popoverFocus of Widgets for the styled painting layer.
func (x *Widgets) GetPopoverFocus() []popoverFocusAsk { return x.popoverFocus }

// GetPopovers reads the popovers of Widgets for the styled painting layer.
func (x *Widgets) GetPopovers() []PopoverDecl { return x.popovers }

// GetRects reads the rects of Widgets for the styled painting layer.
func (x *Widgets) GetRects() []Rect { return x.rects }

// GetRows reads the rows of Widgets for the styled painting layer.
func (x *Widgets) GetRows() []Rect { return x.rows }

// GetSegmented reads the segmented of Widgets for the styled painting layer.
func (x *Widgets) GetSegmented() []segmented { return x.segmented }

// GetSelects reads the selects of Widgets for the styled painting layer.
func (x *Widgets) GetSelects() []SelectDecl { return x.selects }

// GetSliders reads the sliders of Widgets for the styled painting layer.
func (x *Widgets) GetSliders() []SliderDecl { return x.sliders }

// GetSplitButtons reads the splitButtons of Widgets for the styled painting layer.
func (x *Widgets) GetSplitButtons() []SplitButtonDecl { return x.splitButtons }

// GetSplits reads the splits of Widgets for the styled painting layer.
func (x *Widgets) GetSplits() []SplitDecl { return x.splits }

// GetTables reads the tables of Widgets for the styled painting layer.
func (x *Widgets) GetTables() []TableDecl { return x.tables }

// GetTabs reads the tabs of Widgets for the styled painting layer.
func (x *Widgets) GetTabs() []TabsDecl { return x.tabs }

// GetTextareas reads the textareas of Widgets for the styled painting layer.
func (x *Widgets) GetTextareas() []TextareaSnapshot { return x.textareas }

// GetMoved reads the moved of armedButton for the styled painting layer.
func (x *armedButton) GetMoved() bool { return x.moved }

// GetChanged reads the changed of bindDecl for the styled painting layer.
func (x *bindDecl) GetChanged() bool { return x.changed }

// GetField reads the field of bindDecl for the styled painting layer.
func (x *bindDecl) GetField() *Field { return x.field }

// GetFlag reads the flag of bindDecl for the styled painting layer.
func (x *bindDecl) GetFlag() *bool { return x.flag }

// GetIndex reads the index of bindDecl for the styled painting layer.
func (x *bindDecl) GetIndex() *int { return x.index }

// GetSettled reads the settled of bindDecl for the styled painting layer.
func (x *bindDecl) GetSettled() bool { return x.settled }

// GetN reads the n of boundKey for the styled painting layer.
func (x *boundKey) GetN() uint32 { return x.n }

// GetField reads the field of boundText for the styled painting layer.
func (x *boundText) GetField() Field { return x.field }

// GetSynced reads the synced of boundText for the styled painting layer.
func (x *boundText) GetSynced() string { return x.synced }

// GetClear reads the clear of comboIDs for the styled painting layer.
func (x *comboIDs) GetClear() gfx.ID { return x.clear }

// GetDismiss reads the dismiss of comboIDs for the styled painting layer.
func (x *comboIDs) GetDismiss() gfx.ID { return x.dismiss }

// GetFocus reads the focus of comboIDs for the styled painting layer.
func (x *comboIDs) GetFocus() gfx.ID { return x.focus }

// GetInput reads the input of comboIDs for the styled painting layer.
func (x *comboIDs) GetInput() gfx.ID { return x.input }

// GetOptions reads the options of comboIDs for the styled painting layer.
func (x *comboIDs) GetOptions() gfx.Scope { return x.options }

// GetTrigger reads the trigger of comboIDs for the styled painting layer.
func (x *comboIDs) GetTrigger() gfx.ID { return x.trigger }

// GetF reads the f of contextTarget for the styled painting layer.
func (x *contextTarget) GetF() *Frame { return x.f }

// GetMenu reads the menu of contextTarget for the styled painting layer.
func (x *contextTarget) GetMenu() *MenuItems { return x.menu }

// GetTarget reads the target of contextTarget for the styled painting layer.
func (x *contextTarget) GetTarget() int32 { return x.target }

// GetHandlers reads the handlers of controlEntry for the styled painting layer.
func (x *controlEntry) GetHandlers() *Handlers { return x.handlers }

// GetPart reads the part of dragRegion for the styled painting layer.
func (x *dragRegion) GetPart() bool { return x.part }

// GetRegion reads the region of dragSource for the styled painting layer.
func (x *dragSource) GetRegion() dragRegion { return x.region }

// GetAccept reads the accept of dragTarget for the styled painting layer.
func (x *dragTarget) GetAccept() func(source, target gfx.ID) bool { return x.accept }

// GetDrop reads the drop of dragTarget for the styled painting layer.
func (x *dragTarget) GetDrop() func(source, target gfx.ID) { return x.drop }

// GetRegion reads the region of dragTarget for the styled painting layer.
func (x *dragTarget) GetRegion() dragRegion { return x.region }

// GetOver reads the over of drawHooks for the styled painting layer.
func (x *drawHooks) GetOver() func(*DrawContext) { return x.over }

// GetUnder reads the under of drawHooks for the styled painting layer.
func (x *drawHooks) GetUnder() func(*DrawContext) { return x.under }

// GetBody reads the body of drawerState for the styled painting layer.
func (x *drawerState) GetBody() int32 { return x.body }

// GetFooter reads the footer of drawerState for the styled painting layer.
func (x *drawerState) GetFooter() int32 { return x.footer }

// GetLayer reads the layer of drawerState for the styled painting layer.
func (x *drawerState) GetLayer() uint16 { return x.layer }

// GetSpec reads the spec of drawerState for the styled painting layer.
func (x *drawerState) GetSpec() DrawerDeclaration { return x.spec }

// GetVerticalValue reads the accessible value of a scroll area's vertical bar for
// the styled painting layer.
func (x *ScrollState) GetVerticalValue() string { return x.verticalValue }

// GetHorizontalValue reads the accessible value of a scroll area's horizontal bar
// for the styled painting layer.
func (x *ScrollState) GetHorizontalValue() string { return x.horizontalValue }

// GetViewport reads the area the drawer covers, the frame less its spec's Inset.
func (x *drawerState) GetViewport() Rect { return x.viewport }

// GetField reads the field of fieldEntry for the styled painting layer.
func (x *fieldEntry) GetField() *Field { return x.field }

// GetKey reads the key of fieldEntry for the styled painting layer.
func (x *fieldEntry) GetKey() func(gfx.KeyEvent) gfx.EventResult { return x.key }

// GetAxis reads the axis of focusGroup for the styled painting layer.
func (x *focusGroup) GetAxis() FocusAxis { return x.axis }

// GetEnd reads the end of focusGroup for the styled painting layer.
func (x *focusGroup) GetEnd() int32 { return x.end }

// GetOrder reads the order of focusGroup for the styled painting layer.
func (x *focusGroup) GetOrder() FocusOrder { return x.order }

// GetStart reads the start of focusGroup for the styled painting layer.
func (x *focusGroup) GetStart() int32 { return x.start }

// GetAt reads the at of freeSlot for the styled painting layer.
func (x *freeSlot) GetAt() uint32 { return x.at }

// GetSlot reads the slot of freeSlot for the styled painting layer.
func (x *freeSlot) GetSlot() uint32 { return x.slot }

// GetAngle reads the angle of gradient for the styled painting layer.
func (x *gradient) GetAngle() Angle { return x.angle }

// GetCount reads the count of gradient for the styled painting layer.
func (x *gradient) GetCount() int32 { return x.count }

// GetEpoch reads the epoch of groupStop for the styled painting layer.
func (x *groupStop) GetEpoch() uint32 { return x.epoch }

// GetNumber reads the number of intKey for the styled painting layer.
func (x *intKey) GetNumber() int { return x.number }

// GetEpoch reads the epoch of keyEntry for the styled painting layer.
func (x *keyEntry) GetEpoch() uint32 { return x.epoch }

// GetSlot reads the slot of keyEntry for the styled painting layer.
func (x *keyEntry) GetSlot() uint32 { return x.slot }

// GetIsText reads the isText of keyKey for the styled painting layer.
func (x *keyKey) GetIsText() bool { return x.isText }

// GetNumber reads the number of keyKey for the styled painting layer.
func (x *keyKey) GetNumber() int { return x.number }

// GetIndex reads the index of listDeclRow for the styled painting layer.
func (x *listDeclRow) GetIndex() int { return x.index }

// GetKey reads the key of listDeclRow for the styled painting layer.
func (x *listDeclRow) GetKey() int { return x.key }

// GetPinned reads the pinned of listDecl for the styled painting layer.
func (x *listDecl) GetPinned() int32 { return x.pinned }

// GetPinnedRow reads the pinnedRow of listDecl for the styled painting layer.
func (x *listDecl) GetPinnedRow() int { return x.pinnedRow }

// GetRows reads the rows of listDecl for the styled painting layer.
func (x *listDecl) GetRows() int32 { return x.rows }

// GetState reads the state of listDecl for the styled painting layer.
func (x *listDecl) GetState() *ListState { return x.state }

// GetUntil reads the until of listDecl for the styled painting layer.
func (x *listDecl) GetUntil() int32 { return x.until }

// GetActive reads the active of listDrag for the styled painting layer.
func (x *listDrag) GetActive() bool { return x.active }

// GetAt reads the at of listDrag for the styled painting layer.
func (x *listDrag) GetAt() gfx.Point { return x.at }

// GetFrom reads the from of listDrag for the styled painting layer.
func (x *listDrag) GetFrom() int { return x.from }

// GetHandle reads the handle of listDrag for the styled painting layer.
func (x *listDrag) GetHandle() bool { return x.handle }

// GetNextOK reads the nextOK of listDrag for the styled painting layer.
func (x *listDrag) GetNextOK() bool { return x.nextOK }

// GetPressed reads the pressed of listDrag for the styled painting layer.
func (x *listDrag) GetPressed() bool { return x.pressed }

// GetRows reads the rows of listDrag for the styled painting layer.
func (x *listDrag) GetRows() []int { return x.rows }

// GetScrolled reads the scrolled of listDrag for the styled painting layer.
func (x *listDrag) GetScrolled() time.Duration { return x.scrolled }

// GetStart reads the start of listDrag for the styled painting layer.
func (x *listDrag) GetStart() gfx.Point { return x.start }

// GetTo reads the to of listDrag for the styled painting layer.
func (x *listDrag) GetTo() int { return x.to }

// GetValid reads the valid of listDrag for the styled painting layer.
func (x *listDrag) GetValid() bool { return x.valid }

// GetArea reads the area of listInternals for the styled painting layer.
func (x *listInternals) GetArea() int32 { return x.area }

// GetBlock reads the block of listInternals for the styled painting layer.
func (x *listInternals) GetBlock() func(*DrawContext) { return x.block }

// GetChanged reads the changed of listInternals for the styled painting layer.
func (x *listInternals) GetChanged() bool { return x.changed }

// GetChangedAt reads the changedAt of listInternals for the styled painting layer.
func (x *listInternals) GetChangedAt() uint32 { return x.changedAt }

// GetDeclared reads the declared of listInternals for the styled painting layer.
func (x *listInternals) GetDeclared() []listDeclRow { return x.declared }

// GetDrag reads the drag of listInternals for the styled painting layer.
func (x *listInternals) GetDrag() listDrag { return x.drag }

// GetF reads the f of listInternals for the styled painting layer.
func (x *listInternals) GetF() *Frame { return x.f }

// GetFollowFrame reads the followFrame of listInternals for the styled painting layer.
func (x *listInternals) GetFollowFrame() uint8 { return x.followFrame }

// GetLaidOut reads the laidOut of listInternals for the styled painting layer.
func (x *listInternals) GetLaidOut() bool { return x.laidOut }

// GetN reads the n of listInternals for the styled painting layer.
func (x *listInternals) GetN() int { return x.n }

// GetOver reads the over of listInternals for the styled painting layer.
func (x *listInternals) GetOver() func(*DrawContext) { return x.over }

// GetPaint reads the paint of listInternals for the styled painting layer.
func (x *listInternals) GetPaint() func(*DrawContext) { return x.paint }

// GetPending reads the pending of listInternals for the styled painting layer.
func (x *listInternals) GetPending() listScroll { return x.pending }

// GetPendingAt reads the pendingAt of listInternals for the styled painting layer.
func (x *listInternals) GetPendingAt() float32 { return x.pendingAt }

// GetPendingFrame reads the pendingFrame of listInternals for the styled painting layer.
func (x *listInternals) GetPendingFrame() uint8 { return x.pendingFrame }

// GetPendingRow reads the pendingRow of listInternals for the styled painting layer.
func (x *listInternals) GetPendingRow() int32 { return x.pendingRow }

// GetPinned reads the pinned of listInternals for the styled painting layer.
func (x *listInternals) GetPinned() int32 { return x.pinned }

// GetPinnedRow reads the pinnedRow of listInternals for the styled painting layer.
func (x *listInternals) GetPinnedRow() int { return x.pinnedRow }

// GetPinning reads the pinning of listInternals for the styled painting layer.
func (x *listInternals) GetPinning() bool { return x.pinning }

// GetRows reads the rows of listInternals for the styled painting layer.
func (x *listInternals) GetRows() gfx.Scope { return x.rows }

// GetScope reads the scope of listInternals for the styled painting layer.
func (x *listInternals) GetScope() gfx.Scope { return x.scope }

// GetScrollX reads the scrollX of listInternals for the styled painting layer.
func (x *listInternals) GetScrollX() float32 { return x.scrollX }

// GetSelected reads the selected of listInternals for the styled painting layer.
func (x *listInternals) GetSelected() int32 { return x.selected }

// GetSubmitted reads the submitted of listInternals for the styled painting layer.
func (x *listInternals) GetSubmitted() bool { return x.submitted }

// GetSubmittedAt reads the submittedAt of listInternals for the styled painting layer.
func (x *listInternals) GetSubmittedAt() uint32 { return x.submittedAt }

// GetSubmittedRow reads the submittedRow of listInternals for the styled painting layer.
func (x *listInternals) GetSubmittedRow() int32 { return x.submittedRow }

// GetTab reads the tab of listInternals for the styled painting layer.
func (x *listInternals) GetTab() int { return x.tab }

// GetTypeahead reads the typeahead of listInternals for the styled painting layer.
func (x *listInternals) GetTypeahead() MenuTypeahead { return x.typeahead }

// GetKnown reads the known of listTree for the styled painting layer.
func (x *listTree) GetKnown() []int32 { return x.known }

// GetSum reads the sum of listTree for the styled painting layer.
func (x *listTree) GetSum() []float64 { return x.sum }

// GetBefore reads the before of memoBox for the styled painting layer.
func (x *memoBox) GetBefore() gfx.Mark { return x.before }

// GetCheck reads the check of memoBox for the styled painting layer.
func (x *memoBox) GetCheck() int32 { return x.check }

// GetCopy reads the copy of memoBox for the styled painting layer.
func (x *memoBox) GetCopy() bool { return x.copy }

// GetEntry reads the entry of memoBox for the styled painting layer.
func (x *memoBox) GetEntry() *memoEntry { return x.entry }

// GetAt reads the at of memoCheckOf for the styled painting layer.
func (x *memoCheckOf) GetAt() Rect { return x.at }

// GetBounds reads the bounds of memoEntry for the styled painting layer.
func (x *memoEntry) GetBounds() Rect { return x.bounds }

// GetCell reads the cell of memoEntry for the styled painting layer.
func (x *memoEntry) GetCell() any { return x.cell }

// GetEpoch reads the epoch of memoEntry for the styled painting layer.
func (x *memoEntry) GetEpoch() uint32 { return x.epoch }

// GetFonts reads the fonts of memoEntry for the styled painting layer.
func (x *memoEntry) GetFonts() uint32 { return x.fonts }

// GetHeld reads the held of memoEntry for the styled painting layer.
func (x *memoEntry) GetHeld() bool { return x.held }

// GetIntKeys reads the intKeys of memoEntry for the styled painting layer.
func (x *memoEntry) GetIntKeys() []intKey { return x.intKeys }

// GetKeys reads the keys of memoEntry for the styled painting layer.
func (x *memoEntry) GetKeys() []keyKey { return x.keys }

// GetListens reads the listens of memoEntry for the styled painting layer.
func (x *memoEntry) GetListens() bool { return x.listens }

// GetMemos reads the memos of memoEntry for the styled painting layer.
func (x *memoEntry) GetMemos() []*memoEntry { return x.memos }

// GetNames reads the names of memoEntry for the styled painting layer.
func (x *memoEntry) GetNames() []gfx.ID { return x.names }

// GetPainted reads the painted of memoEntry for the styled painting layer.
func (x *memoEntry) GetPainted() bool { return x.painted }

// GetScale reads the scale of memoEntry for the styled painting layer.
func (x *memoEntry) GetScale() float32 { return x.scale }

// GetStale reads the stale of memoEntry for the styled painting layer.
func (x *memoEntry) GetStale() bool { return x.stale }

// GetThemeEpoch reads the themeEpoch of memoEntry for the styled painting layer.
func (x *memoEntry) GetThemeEpoch() uint32 { return x.themeEpoch }

// GetAt reads the at of menuOrigin for the styled painting layer.
func (x *menuOrigin) GetAt() Rect { return x.at }

// GetOpen reads the open of menuOrigin for the styled painting layer.
func (x *menuOrigin) GetOpen() *bool { return x.open }

// GetBorn reads the born of nameUse for the styled painting layer.
func (x *nameUse) GetBorn() uint32 { return x.born }

// GetSeen reads the seen of nameUse for the styled painting layer.
func (x *nameUse) GetSeen() uint32 { return x.seen }

// GetScope reads the scope of popoverFocusAsk for the styled painting layer.
func (x *popoverFocusAsk) GetScope() gfx.ID { return x.scope }

// GetBorn reads the born of protoBound for the styled painting layer.
func (x *protoBound) GetBorn() uint32 { return x.born }

// GetEpoch reads the epoch of protoBound for the styled painting layer.
func (x *protoBound) GetEpoch() uint32 { return x.epoch }

// GetPositional reads the positional of protoBound for the styled painting layer.
func (x *protoBound) GetPositional() bool { return x.positional }

// GetIndex reads the index of protoName for the styled painting layer.
func (x *protoName) GetIndex() int { return x.index }

// GetKey reads the key of protoName for the styled painting layer.
func (x *protoName) GetKey() keyKey { return x.key }

// GetKindOf reads the kindOf of protoName for the styled painting layer.
func (x *protoName) GetKindOf() uint8 { return x.kindOf }

// GetPart reads the part of protoName for the styled painting layer.
func (x *protoName) GetPart() string { return x.part }

// GetCounts reads the counts of protoNode for the styled painting layer.
func (x *protoNode) GetCounts() [pkKinds]uint32 { return x.counts }

// GetNamed reads the named of protoNode for the styled painting layer.
func (x *protoNode) GetNamed() bool { return x.named }

// GetParentNode reads the parentNode of protoNode for the styled painting layer.
func (x *protoNode) GetParentNode() int32 { return x.parentNode }

// GetParentScope reads the parentScope of protoNode for the styled painting layer.
func (x *protoNode) GetParentScope() gfx.Scope { return x.parentScope }

// GetScope reads the scope of protoNode for the styled painting layer.
func (x *protoNode) GetScope() gfx.Scope { return x.scope }

// GetEpoch reads the epoch of protoScope for the styled painting layer.
func (x *protoScope) GetEpoch() uint32 { return x.epoch }

// GetKeySub reads the keySub of protoScope for the styled painting layer.
func (x *protoScope) GetKeySub() gfx.Scope { return x.keySub }

// GetKeys reads the keys of protoScope for the styled painting layer.
func (x *protoScope) GetKeys() slotPool { return x.keys }

// GetLabels reads the labels of protoScope for the styled painting layer.
func (x *protoScope) GetLabels() []string { return x.labels }

// GetPrev reads the prev of protoScope for the styled painting layer.
func (x *protoScope) GetPrev() []string { return x.prev }

// GetPrevEpoch reads the prevEpoch of protoScope for the styled painting layer.
func (x *protoScope) GetPrevEpoch() uint32 { return x.prevEpoch }

// GetN reads the n of protoStateKey for the styled painting layer.
func (x *protoStateKey) GetN() uint32 { return x.n }

// GetEpoch reads the epoch of protoState for the styled painting layer.
func (x *protoState) GetEpoch() uint32 { return x.epoch }

// GetBorder reads the border of resolved for the styled painting layer.
func (x *resolved) GetBorder() Color { return x.border }

// GetDashed reads the dashed of resolved for the styled painting layer.
func (x *resolved) GetDashed() bool { return x.dashed }

// GetRing reads the ring of resolved for the styled painting layer.
func (x *resolved) GetRing() Color { return x.ring }

// GetRingWidth reads the ringWidth of resolved for the styled painting layer.
func (x *resolved) GetRingWidth() float32 { return x.ringWidth }

// GetShadow reads the shadow of resolved for the styled painting layer.
func (x *resolved) GetShadow() gfx.ShadowLevel { return x.shadow }

// GetF reads the f of rover for the styled painting layer.
func (x *rover) GetF() *Frame { return x.f }

// GetTabs reads the tabs of rover for the styled painting layer.
func (x *rover) GetTabs() bool { return x.tabs }

// GetAbs reads the abs of scratch for the styled painting layer.
func (x *scratch) GetAbs() []int32 { return x.abs }

// GetCells reads the cells of scratch for the styled painting layer.
func (x *scratch) GetCells() []bool { return x.cells }

// GetRowAt reads the rowAt of scratch for the styled painting layer.
func (x *scratch) GetRowAt() []float32 { return x.rowAt }

// GetRows reads the rows of scratch for the styled painting layer.
func (x *scratch) GetRows() []float32 { return x.rows }

// GetSeen reads the seen of scrollSlot for the styled painting layer.
func (x *scrollSlot) GetSeen() uint32 { return x.seen }

// GetState reads the state of scrollSlot for the styled painting layer.
func (x *scrollSlot) GetState() ScrollState { return x.state }

// GetEpoch reads the epoch of scrollStore for the styled painting layer.
func (x *scrollStore) GetEpoch() uint32 { return x.epoch }

// GetFree reads the free of scrollStore for the styled painting layer.
func (x *scrollStore) GetFree() []*scrollSlot { return x.free }

// GetSlots reads the slots of scrollStore for the styled painting layer.
func (x *scrollStore) GetSlots() map[gfx.ID]*scrollSlot { return x.slots }

// GetOptions reads the options of segmented for the styled painting layer.
func (x *segmented) GetOptions() []style.SegmentOption { return x.options }

// GetSelected reads the selected of segmented for the styled painting layer.
func (x *segmented) GetSelected() gfx.ID { return x.selected }

// GetChar reads the char of shortcutEntry for the styled painting layer.
func (x *shortcutEntry) GetChar() byte { return x.char }

// GetFocus reads the focus of shortcutEntry for the styled painting layer.
func (x *shortcutEntry) GetFocus() gfx.ID { return x.focus }

// GetKey reads the key of shortcutEntry for the styled painting layer.
func (x *shortcutEntry) GetKey() gfx.Key { return x.key }

// GetModifiers reads the modifiers of shortcutEntry for the styled painting layer.
func (x *shortcutEntry) GetModifiers() gfx.Modifiers { return x.modifiers }

// GetRows reads the rows of shortcutEntry for the styled painting layer.
func (x *shortcutEntry) GetRows() gfx.Scope { return x.rows }

// GetScope reads the scope of shortcutEntry for the styled painting layer.
func (x *shortcutEntry) GetScope() gfx.Scope { return x.scope }

// GetFree reads the free of slotPool for the styled painting layer.
func (x *slotPool) GetFree() []freeSlot { return x.free }

// GetAnimated reads the animated of stateLooks for the styled painting layer.
func (x *stateLooks) GetAnimated() bool { return x.animated }

// GetFocus reads the focus of stateLooks for the styled painting layer.
func (x *stateLooks) GetFocus() Look { return x.focus }

// GetFollow reads the follow of stateLooks for the styled painting layer.
func (x *stateLooks) GetFollow() int32 { return x.follow }

// GetHover reads the hover of stateLooks for the styled painting layer.
func (x *stateLooks) GetHover() Look { return x.hover }

// GetMotion reads the motion of stateLooks for the styled painting layer.
func (x *stateLooks) GetMotion() gfx.Motion { return x.motion }

// GetPressed reads the pressed of stateLooks for the styled painting layer.
func (x *stateLooks) GetPressed() Look { return x.pressed }

// GetScope reads the scope of stateName for the styled painting layer.
func (x *stateName) GetScope() gfx.Scope { return x.scope }

// GetBar reads the bar of tableRoute for the styled painting layer.
func (x *tableRoute) GetBar() gfx.ID { return x.bar }

// GetBounds reads the bounds of tableRoute for the styled painting layer.
func (x *tableRoute) GetBounds() Rect { return x.bounds }

// GetHorizontal reads the horizontal of tableRoute for the styled painting layer.
func (x *tableRoute) GetHorizontal() gfx.ID { return x.horizontal }

// GetSideways reads the sideways of tableRoute for the styled painting layer.
func (x *tableRoute) GetSideways() bool { return x.sideways }

// GetState reads the state of tableRoute for the styled painting layer.
func (x *tableRoute) GetState() *TableState { return x.state }

// GetAction reads the action of toast for the styled painting layer.
func (x *toast) GetAction() string { return x.action }

// GetDescription reads the description of toast for the styled painting layer.
func (x *toast) GetDescription() string { return x.description }

// GetN reads the n of toast for the styled painting layer.
func (x *toast) GetN() uint32 { return x.n }

// GetShown reads the shown of toast for the styled painting layer.
func (x *toast) GetShown() bool { return x.shown }

// GetTitle reads the title of toast for the styled painting layer.
func (x *toast) GetTitle() string { return x.title }

// GetActive reads the active of treeDrag for the styled painting layer.
func (x *treeDrag) GetActive() bool { return x.active }

// GetAt reads the at of treeDrag for the styled painting layer.
func (x *treeDrag) GetAt() gfx.Point { return x.at }

// GetFrom reads the from of treeDrag for the styled painting layer.
func (x *treeDrag) GetFrom() int { return x.from }

// GetNextOK reads the nextOK of treeDrag for the styled painting layer.
func (x *treeDrag) GetNextOK() bool { return x.nextOK }

// GetPressed reads the pressed of treeDrag for the styled painting layer.
func (x *treeDrag) GetPressed() bool { return x.pressed }

// GetScrolled reads the scrolled of treeDrag for the styled painting layer.
func (x *treeDrag) GetScrolled() time.Duration { return x.scrolled }

// GetStart reads the start of treeDrag for the styled painting layer.
func (x *treeDrag) GetStart() gfx.Point { return x.start }

// GetTarget reads the target of treeDrag for the styled painting layer.
func (x *treeDrag) GetTarget() TreeMove { return x.target }

// GetValid reads the valid of treeDrag for the styled painting layer.
func (x *treeDrag) GetValid() bool { return x.valid }

// GetFilled reads the filled of wrapCache for the styled painting layer.
func (x *wrapCache) GetFilled() bool { return x.filled }

// GetMono reads the mono of wrapCache for the styled painting layer.
func (x *wrapCache) GetMono() bool { return x.mono }

// GetWidth reads the width of wrapCache for the styled painting layer.
func (x *wrapCache) GetWidth() float32 { return x.width }

// KeysSlice is the menu's keycap scratch as a slice.
func (x *MenuItems) KeysSlice() []string { return x.keys[:] }

// SetTooltipValue writes the tooltip state of Frame.
func (x *Frame) SetTooltipValue(v any) { x.tooltip = v }

// SetBarOffset writes the scrollbar offset of ScrollState.
func (x *ScrollState) SetBarOffset(v float32) { x.barOffset = v }

// SetDraggingState writes the dragging flag of SliderState.
func (x *SliderState) SetDraggingState(v bool) { x.dragging = v }

// SetGrab writes the grab of SliderState.
func (x *SliderState) SetGrab(v float32) { x.grab = v }

// SetStack writes the stack of Toaster.
func (x *Toaster) SetStack(v []SonnerToast) { x.stack = v }

// SetActed writes the acted toast of Toaster.
func (x *Toaster) SetActed(v ToastID) { x.acted = v }

// SpecPtr is the drawer declaration, for its identities.
func (x *drawerState) SpecPtr() *DrawerDeclaration { return &x.spec }

// SetScope writes the toaster scope.
func (x *Toaster) SetScope(v gfx.Scope) { x.scope = v }

// SetCells writes the cells of Widgets.
func (x *Widgets) SetCells(v []string) { x.cells = v }

// SetLayer writes the layer of Widgets.
func (x *Widgets) SetLayer(v uint16) { x.layer = v }

// SetPopoverFocus writes the popover focus scratch of Widgets.
func (x *Widgets) SetPopoverFocus(v []popoverFocusAsk) { x.popoverFocus = v }

// SetOpenState is set by the styled painting layer.
func (x *ColorPickerState) SetOpenState(v bool) { x.open = v }

// SetRestoreFocusState is set by the styled painting layer.
func (x *ColorPickerState) SetRestoreFocusState(v bool) { x.restoreFocus = v }

// SetLayoutValue is set by the styled painting layer.
func (x *ColorPickerState) SetLayoutValue(v ColorPickerLayout) { x.layout = v }

// SetAppliedValue is set by the styled painting layer.
func (x *ColorPickerState) SetAppliedValue(v bool) { x.applied = v }

// SetToggledIndex is set by the styled painting layer.
func (x *TreeState) SetToggledIndex(v int) { x.toggled = v }

// SetChosenIndex is set by the styled painting layer.
func (x *TreeState) SetChosenIndex(v int) { x.chosen = v }

// SetDeclaredRows is set by the styled painting layer.
func (x *TreeState) SetDeclaredRows(v []TreeRow) { x.declared = v }

// SetBarSinceTime is set by the styled painting layer.
func (x *ScrollState) SetBarSinceTime(v time.Duration) { x.barSince = v }

// SetBarSeenTime is set by the styled painting layer.
func (x *ScrollState) SetBarSeenTime(v bool) { x.barSeen = v }

// SetBarOffsetXValue is set by the styled painting layer.
func (x *ScrollState) SetBarOffsetXValue(v float32) { x.barOffsetX = v }

// SetClearLabel is set by the styled painting layer.
func (x *ComboState) SetClearLabel(v string) { x.clearLabel = v }

// SetClearLabelOf is set by the styled painting layer.
func (x *ComboState) SetClearLabelOf(v string) { x.clearLabelOf = v }

// SetDialogs is set by the styled painting layer.
func (x *Widgets) SetDialogs(v []DialogDecl) { x.dialogs = v }

// SetPopovers is set by the styled painting layer.
func (x *Widgets) SetPopovers(v []PopoverDecl) { x.popovers = v }

// SetRows is set by the styled painting layer.
func (x *Widgets) SetRows(v []Rect) { x.rows = v }

// SetRects is set by the styled painting layer.
func (x *Widgets) SetRects(v []Rect) { x.rects = v }

// SetIcon is set by the styled painting layer.
func (x *Extra) SetIcon(v gfx.Icon) { x.icon = v }

// SetStart is set by the styled painting layer.
func (x *FieldAddons) SetStart(v InputGroupSlot) { x.start = v }

// SetBarLit is set by the styled painting layer.
func (x *ScrollState) SetBarLit(v bool) { x.barLit = v }

// SetScratch is read or set by the styled painting layer.
func (x *ComboState) SetScratch(v []ComboboxOption) { x.scratch = v }

// SetOpts is read or set by the styled painting layer.
func (x *ComboState) SetOpts(v []SingleSelectOption) { x.opts = v }

// SetDescs is read or set by the styled painting layer.
func (x *ComboState) SetDescs(v []ComboboxOption) { x.descs = v }

// SetArea is read or set by the styled painting layer.
func (x *SliderState) SetArea(v Rect) { x.area = v }

// SetFocusFirst is read or set by the styled painting layer.
func (x *MenuItems) SetFocusFirst(v bool) { x.focusFirst = v }

// SetShown is read or set by the styled painting layer.
func (x *toast) SetShown(v bool) { x.shown = v }

// GetKey is read or set by the styled painting layer.
func (x *TreeState) GetKey() func(i int) int { return x.key }

// GetAppliedValue is read or set by the styled painting layer.
func (x *ColorPickerState) GetAppliedValue() bool { return x.applied }

// SpecPtr is used by the styled painting layer.
func (x *AlertDialogDecl) SpecPtr() *AlertDialogSpec { return &x.spec }

// IDsPtr is used by the styled painting layer.
func (x *ComboState) IDsPtr() *comboIDs { return &x.ids }

// ListPtr is used by the styled painting layer.
func (x *ComboState) ListPtr() *SingleSelectState { return &x.list }

// HexPtr is used by the styled painting layer.
func (x *ColorPickerState) HexPtr() *Field { return &x.hex }

// QueuePtr is used by the styled painting layer.
func (x *Toaster) QueuePtr() *ToastQueue { return &x.queue }

// DragPtr is used by the styled painting layer.
func (x *TreeState) DragPtr() *treeDrag { return &x.drag }

// SetEnd is used by the styled painting layer.
func (x *FieldAddons) SetEnd(v InputGroupSlot) { x.end = v }

// SetHeader is used by the styled painting layer.
func (x *DialogDecl) SetHeader(v int32) { x.header = v }

// SetBody is used by the styled painting layer.
func (x *DialogDecl) SetBody(v int32) { x.body = v }

// SetFooter is used by the styled painting layer.
func (x *DialogDecl) SetFooter(v int32) { x.footer = v }

// NewFromSpec is used by the styled painting layer.
func (x *DialogDecl) NewFromSpec(spec DialogSpec, layer int) DialogDecl {
	return DialogDecl{spec: spec, layer: uint16(layer)}
}

// NewDialogDecl makes a dialog declaration from its spec.
func NewDialogDecl(spec DialogSpec, layer uint16) DialogDecl {
	return DialogDecl{spec: spec, layer: layer}
}

// SetPaintPopoverFn is used by the styled painting layer.
func (x *Frame) SetPaintPopoverFn(v func(f *Frame, i int32, n *Node)) { x.paintPopoverFn = v }

// SpecPtr is used by the styled painting layer.
func (x *PopoverDecl) SpecPtr() *PopoverSpec { return &x.spec }

// NewPopoverDecl makes a popover declaration.
func NewPopoverDecl(spec PopoverSpec, anchor int32, layer uint16) PopoverDecl {
	return PopoverDecl{spec: spec, Anchor: anchor, layer: layer}
}

// SetRects is set by the styled painting layer.
func (x *SelectDecl) SetRects(v int32) { x.rects = v }

// NewTreeRow makes a tree row declaration.
func NewTreeRow(index, key int, node int32, gen uint32, item style.TreeItem) TreeRow {
	return TreeRow{index: index, key: key, Node: node, gen: gen, item: item}
}

// SetRectsCells writes the cell rectangle scratch of Widgets.
func (x *Widgets) SetRectsCells(v []Rect) { x.rects = v }
