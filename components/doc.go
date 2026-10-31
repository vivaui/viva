// Package components contains source-oriented UI recipes built from ui
// primitives.
//
// Every recipe is a plain Go value: one component per file, no runtime
// registry, no reflection, no package init, no hidden workers. Linkage is
// pay-for-what-you-use — the linker keeps only the functions an application
// actually calls, and a blank import of this whole package links no symbol of
// github.com/vivaui/viva/components (linkage_test.go builds that program and checks); the
// standard-library packages and github.com/vivaui/viva/ui it imports add a fixed baseline that is
// measured separately. Each file is both the
// implementation and the documentation:
// colors, geometry, metrics, and state rendering are visible in the source,
// so a reader (human or model) can understand the full behavior from that
// file alone.
//
// # Declaration pattern
//
// Components are stateless immediate declarations over a shared ui.Canvas. A
// Begin constructor returns a small chainable value whose terminal method
// paints; the application owns all state and re-declares on repaint:
//
//	BeginButton(canvas, theme, id, bounds).Default("Save")
//
// Hit declaration, hover/press/focus feedback, focus rings, and semantic
// labels are built into each recipe. Keep hit IDs as explicit literals in
// disjoint ranges and stable across paints so retained scenes, focus, and
// accessibility targets stay stable.
//
// Editing components (input, textarea, OTP, combobox) render caller-owned
// snapshots such as InputSnapshot: value, caret, selection, and IME
// composition offsets go in, painted glyphs come out. Editing state itself
// lives in the application, typically driven by an editor from github.com/vivaui/viva/text.
//
// # Customization
//
// Two levels, in order. Chain knobs (Height, Tint, Disabled, SizeToFit, and
// friends) cover common adjustments without leaving the package. For deeper
// change the file is the framework — install a copy into the application
// module and edit it freely:
//
//	viva add button
//
// Installed sources are the primary customization path, and every file of this
// package is a recipe: viva list prints their names. A recipe that calls into
// another file (a shared color helper, a menu surface) is installed together
// with it. The installable copies are generated from these files by
// tools/componentgen, so they cannot drift; go generate ./cmd/viva refreshes
// them after a component changes. An installed copy stops receiving fixes made
// here, so import the package until a recipe needs more than its knobs.
//
// # Catalog
//
// Multi-selection:
//   - MultiSelect: `PaintMultiSelect` combines removable chips, a native query
//     input and a filtered popup. `MultiSelectState` handles multi-selection,
//     optional item creation, limits and keyboard/IME-safe confirmation. Bind
//     its Query editor to the application's text-input provider.
//
// Actions:
//   - Button: `BeginButton` paints a labeled control with Default, Secondary,
//     Outline, and Ghost variants; hover, press, and focus states, metrics-
//     scaled geometry, and the hit declaration are built in.
//   - Button row: `BeginButtonRow` lays small buttons out against the right
//     edge of a footer, each as wide as its label, shrinking together when
//     they do not fit.
//   - Button group: `BeginButtonGroup` joins a row of buttons into one
//     bordered control.
//   - Split button: `BeginSplitButton` joins a primary action to a fixed-width,
//     independently focusable menu trigger while sharing one visual surface.
//   - Toggle: `BeginToggle` paints a two-state icon button.
//   - Toggle group: `BeginToggleGroup` paints a set of selectable toggles;
//     `ToggleGroupKeyDown` wires arrow-key movement.
//   - Segmented control: `BeginSegmentedControl` paints a compact
//     single-select strip; `SegmentedControlKeyDown` wires arrow keys.
//   - Switch: `BeginSwitch` paints the on/off track and knob.
//   - Checkbox: `BeginCheckbox` paints the checked/unchecked box and check
//     mark.
//   - Color picker: `BeginColorPicker` paints an anchored HSV panel, optional
//     alpha slider, caller-owned HEX input, and clear/cancel/apply actions.
//     `ColorPickerLayout.Pointer` and `ColorPickerLayout.Key` update HSV drafts.
//   - Color swatch: `BeginColorSwatch` paints a named circular color choice,
//     with selection, focus and disabled states and a no-color option.
//   - Radio group: `BeginRadioGroup` paints single-select labeled options;
//     `RadioGroupKeyDown` wires arrow keys.
//   - Slider: `BeginSlider` paints one draggable thumb; `BeginRangeSlider`
//     paints several.
//
// Inputs:
//   - Input: `PaintInput` paints the single-line editor recipe from an
//     InputSnapshot, including caret, selection, placeholder, and IME
//     composition.
//   - Textarea: `PaintTextarea` paints the multi-line editor recipe, wrapping
//     long lines; `TextareaCommandForKey` maps key events to editor commands,
//     and `TextareaMovementForKey` with a `TextareaLayout` moves the caret by
//     visual row (arrows, Home, End, Page keys), scrolls to it, places it on a
//     click and sizes a box that grows with its text.
//   - Code editor: `CodeEditor` adds caller-provided syntax colors and a draggable
//     scrollbar to TextareaLayout, retaining native selection and IME geometry.
//   - Code diff: `CodeDiff` paints an aligned, syntax-highlighted comparison
//     with one read-only source and an editable right source; `DiffCodeLines`
//     produces its bounded line alignment and statistics.
//   - Image preview: `PaintImageView` clips an immutable bitmap with fit, zoom
//     and pan; actual size maps one bitmap pixel to one device pixel at any DPI.
//   - One-time code: `PaintInputOTP` paints segmented code entry;
//     `InputOTPCommandForKey` maps key events to editor commands.
//   - Input group: `PaintInputGroup` paints an input with leading and
//     trailing text, button, key-cap, or icon addons.
//   - Select: `BeginSelect` paints the trigger and filtered list popup.
//   - Single select: `PaintSingleSelect` combines one selected value, a native
//     search editor and a filtered popup. `SingleSelectState` supports optional
//     item creation, clearing, keyboard navigation and IME-safe confirmation.
//   - Native select: `PaintNativeSelect` paints the platform-styled control
//     and `PaintNativeSelectSemantics` declares its accessibility tree;
//     `NativeSelectCommandForKey` maps key events.
//   - Combobox: `BeginCombobox` paints an editable input with a suggestion
//     list; `PaintComboboxChips` is the wrapping chip container of a
//     multi-select.
//   - Field: `BeginField` wraps a label, control slot, and hint;
//     `FieldLegend` and `FieldSeparator` structure larger forms, and
//     `BeginFieldSet` and `BeginFieldGroup` stack their rows at Vega's gaps.
//     Naming the control (`Field.For`, `FieldTitle`, `FieldContent`) makes
//     pressing the title focus and click it, as a label's `htmlFor` does, which
//     is how the words beside a switch or a checkbox toggle it.
//   - Form field: `PaintFormField` composes label, input, and validation
//     message from one snapshot.
//
// Layout:
//   - Card: `Card` paints the default card surface; `PaintCard` selects a
//     size tier and returns the header, content and footer rectangles, and
//     `CardSpacing` returns the matching content inset.
//   - Metric card: `MetricCard` paints a title, value, and trend summary.
//   - Separator: `PaintSeparator` paints an auto-direction hairline.
//   - Aspect ratio: `AspectRatio` computes the rect for content of a given
//     ratio inside bounds.
//   - Resizable: `BeginResizable` paints a two-panel split with a draggable
//     handle.
//   - Reorder list: `BeginReorderList` reserves independent drag handles and
//     content slots. `ReorderListState` returns completed pointer/keyboard moves
//     while the application keeps ownership of the ordered data.
//   - Tree: `BeginTree` paints caller-flattened hierarchical rows with stable
//     disclosure and selection semantics while leaving content application-owned.
//   - Scroll area: `BeginScrollArea` paints a clipped viewport and scrollbar
//     for caller-owned content; Viewport makes it focusable and
//     ScrollForKey moves the caller's offsets.
//   - Sidebar: `BeginSidebar` paints the app sidebar shell, or
//     `BeginSidebarWithOptions` with explicit options; it takes header,
//     group, menu item (with badge and action), sub menu and footer rows, and
//     `PaintSidebarInset` paints the page card of the Inset variant.
//   - Item: `BeginItem` paints a neutral list row surface and returns its media
//     box and text column; `PaintItemSeparator` paints the divider between rows.
//   - Collapsible: `BeginCollapsible` paints a disclosure trigger; the
//     caller paints content below it while open.
//   - Accordion: `BeginAccordion` paints stacked disclosure rows; Item
//     returns each content rectangle.
//
// Overlays enter and leave: while the application paints one it fades
// and moves in, and once it stops the canvas draws its last frame out, with no
// input. A menu, select or combobox has to call End for that.
//
// Overlays:
//   - Dialog: `BeginDialog` paints a centered modal with focus scope and
//     dismiss handling; `Dialog.Actions` lays its footer buttons out.
//   - Alert dialog: `BeginAlertDialog` paints the confirmation modal;
//     `AlertDialogOptions.ActionVariant` styles the confirming button, e.g.
//     `ButtonDestructive`.
//   - Drawer: `BeginDrawer` paints a side-anchored overlay panel.
//   - Sheet: `BeginSheet` paints the slide-over panel variant.
//   - Popover: `BeginPopover` paints a panel anchored to a trigger rect;
//     `PlaceOverlay` is the placement (side, align, flip, clamp) every
//     anchored overlay shares and a menu can call.
//   - Hover card: `BeginHoverCard` paints a preview card anchored to a
//     target rect; `HoverIntent` supplies the hover rest, and an ID plus
//     DismissID make the card enterable and dismissible.
//   - Tooltip: `BeginTooltip` paints a label anchored to a target rect;
//     `HoverTooltip` owns hover rest and, with ShowOnFocus, keyboard focus.
//   - Dropdown menu: `BeginDropdownMenu` paints a menu opened from a
//     trigger. A menu or command list taller than its bounds scrolls by the
//     caller's `MenuScroll`, which takes the wheel, moves the arrow, Home, End
//     and Page keys through the items (scrolling the one it reaches into
//     view), matches a typed prefix through `MenuTypeahead`, and drags the
//     scrollbar when it has an identity.
//   - Context menu: `BeginContextMenu` paints a menu at the pointer over a
//     region.
//   - Command: `BeginCommand` paints the searchable command palette
//     surface.
//
// Feedback:
//   - Alert: `Alert` paints an inline banner with variant, optional icon,
//     title, and wrapped description; `AlertHeight` measures it.
//   - Toast: `PaintToast` paints one caller-owned toast.
//   - Toast stack: `PaintSonner` paints the stacked toast column;
//     `ToastQueue` owns Base UI's timeout, visible limit, loading exemption and
//     pause/resume, while the caller owns swipe and the exit.
//   - Progress: `Progress` paints a determinate bar for a fraction,
//     `ProgressIndeterminate` a sliding segment for unknown length, and
//     `PaintProgressRow` adds the label and value line above the track.
//   - Spinner: `PaintSpinner` paints the loading indicator for a phase.
//   - Skeleton: `BeginSkeleton` paints a loading placeholder block.
//   - Empty: `Empty` paints a centered empty state with line icon, title,
//     and description.
//
// Navigation:
//   - Breadcrumb: `BeginBreadcrumb` paints a hierarchy path.
//   - Menubar: `BeginMenubar` paints the top-level bar and
//     `BeginMenubarContent` paints the open menu.
//   - Navigation menu: `BeginNavigationMenu` paints the navbar with peek
//     panels.
//   - Pagination: `BeginPagination` paints page controls.
//   - Tabs: `BeginTabs` paints the tab strip; `TabsKeyDown` wires arrow
//     keys. `BeginTabList` numbers its own tabs and `Paint().Clicked()` reports
//     the one clicked. Fit() sizes the list to its labels.
//
// Data display:
//   - Table: `BeginTable` paints the column-driven table shell; its rows can
//     number themselves and report the one clicked. A cell wider than its
//     column ends in an ellipsis; `FitTableHeadings` widens fixed columns to
//     hold their headings.
//   - Calendar: `BeginCalendar` paints the month grid; selection state is
//     caller-owned and `CalendarCommandForKey` maps the navigation keys.
//   - Carousel: `BeginCarousel` paints a pager viewport for caller content.
//   - Chart: `PaintChartContainer` paints the legend and returns the plot
//     area; `PaintChartLegend` and `PaintChartTooltip` paint the matching
//     chrome. `PaintAreaChart` and `PaintBarChart` paint a complete chart
//     (axes, grid, series, hover mark and tooltip) from typed slices;
//     `ChartState` holds the hovered point and the label caches,
//     `ChartStepForKey` maps the arrow keys, and `ChartDownsample` reduces a
//     long series to the points a plot can show.
//   - Message: `Message` paints one chat row; `BeginMessageGroup` stacks the
//     rows of consecutive messages; `BeginMessageScroller` paints the
//     scrollback viewport with jump controls, and `MessageScrollState` holds
//     its policy (follow the live edge, anchor a turn, keep place when history
//     is prepended).
//   - Bubble: `Bubble` paints one chat bubble that shrinks to its text and
//     wraps; `BubbleReaction` paints its reaction chip.
//   - Attachment: `BeginAttachment` paints the media/upload card for a
//     state, with icon actions; `BeginAttachmentGroup` lays cards in a row.
//   - Badge: `PaintBadge` paints the semantic status pill and returns the
//     consumed rect.
//   - Marker: `Marker` paints the plain, centered-separator, and
//     bottom-border marker forms; `MarkerProgress` puts a spinner in the icon slot.
//   - Questionnaire: `BeginQuestionnaire` paints a question list with radio
//     or checkbox semantics.
//   - Avatar: `Avatar` paints one user glyph; `BeginAvatarGroup` overlaps a
//     row of them.
//
// Typography:
//   - Label: `BeginLabel` paints themed text with alignment options;
//     `LabelPainter.For` makes it the label of a control (`htmlFor`).
//   - Keycap: `Kbd` paints one keyboard key chip; `BeginKbdGroup` lays out
//     a sequence.
//   - Typography: `Typography` paints the heading and body text scale.
//   - Markdown: `MarkdownView` lays out and paints a parsed `markdown.Document`
//     (headings, lists, quotes, code blocks, tables, bold, links) in a column,
//     with the link regions and code blocks it contains.
//
// Foundation:
//   - Direction: `Direction` selects LTR or RTL inline flow
//     (`DirectionLTR`, `DirectionRTL`); `LogicalRect` describes geometry on
//     logical axes.
package components
