package main

import "github.com/vivaui/viva/internal/componentdemo"

func documentation() map[string]Page {
	pages := make(map[string]Page, len(componentdemo.Catalog()))
	for _, component := range componentdemo.Catalog() {
		pages[component.ID] = Page{Sections: []Section{
			{ID: "usage", Title: "Source-owned usage", Text: "This preview calls the production Go component directly. Stable IDs, values and open state remain owned by the application; the component only paints and declares interaction semantics.", Items: []string{"Read the linked component source for every option and visual decision.", "Use the native command to verify system text, focus, accessibility and platform integration.", "The browser preview validates shared layout and scene behavior, not native performance."}},
			{ID: "verification", Title: "Verification boundary", Text: "The examples cover representative states supported by the current API. They do not imply that every desktop backend has completed hardware validation."},
		}}
	}
	overrides := map[string]Page{
		"button": {Sections: []Section{
			{ID: "usage", Title: "Usage", Text: "flex.Frame.Button declares a labeled hit region and paints a source-owned button. Keep the stable ID across repaints; Clicked reports activation in the same declaration.", Items: []string{"Default, Secondary, Outline, Ghost, Destructive and Link share the same interaction model.", "Buttons measure their label by default; set Width to reserve a fixed size.", "XSmall, Small, Large and icon sizes are explicit options. Icon-only controls still need an accessible name."}},
			{ID: "keyboard", Title: "Keyboard & state", Text: "Tab moves focus; Enter or Space activates the focused enabled button. Pointer focus does not paint a keyboard focus ring.", Items: []string{"Disabled controls cannot activate and are excluded from focus traversal.", "Hover, press and focus are read from Canvas, not stored as duplicate application state."}},
		}},
		"badge": {Sections: []Section{
			{ID: "usage", Title: "Content-sized by default", Text: "PaintBadge measures its text and returns the resulting bounds. Supply an origin rather than stretching the badge across a container.", Items: []string{"Use Default for emphasis, Secondary for quieter status, Outline for low emphasis and Destructive for a failure state.", "The long-label example includes mixed Chinese and Latin text; system font metrics may differ between platforms."}},
			{ID: "keyboard", Title: "Semantics", Text: "A badge is a label, not a button. It does not accept focus or create a click target. Use a button when the element performs an action."},
		}},
		"checkbox": {Sections: []Section{
			{ID: "usage", Title: "Controlled selection", Text: "The application owns the checked boolean. BeginCheckbox reads it, declares checkbox semantics and paints the current state; the click callback changes that boolean.", Items: []string{"Give each checkbox a stable ID and an explicit label.", "Disabled and Invalid are presentation and interaction options; they do not own validation state.", "Indeterminate is not implemented by the current checkbox API; it is not simulated in the demo."}},
			{ID: "keyboard", Title: "Keyboard & accessibility", Text: "Tab focuses an enabled checkbox; Space or Enter triggers the same caller-owned toggle as a click. The checked state is exposed through the semantic tree."},
		}},
		"switch": {Sections: []Section{
			{ID: "usage", Title: "An explicit on/off setting", Text: "BeginSwitch takes the application-owned checked value. Label names the control; Small changes its compact geometry without creating a different state model.", Items: []string{"Use switches for settings that take effect immediately, not confirmation actions.", "Disabled switches remain visible but do not change state.", "Browser previews show the final state; native transition timing must be checked in the native preview."}},
			{ID: "keyboard", Title: "Keyboard & state", Text: "Tab reaches an enabled switch. Enter and Space toggle the setting through the click handler. Theme colors come from typed tokens."},
		}},
		"input": {Sections: []Section{
			{ID: "usage", Title: "Text stays with the application", Text: "PaintInput paints a field snapshot and its hit region. Web and native text bridges connect to the same caller-owned Go editor, which retains text, selection and composition while each platform supplies text-system events.", Items: []string{"Only hide the static label while the field actually owns platform editing focus.", "On blur, publish the current editor value as a label in the same frame that disables the platform editor.", "InputSnapshot selection offsets are UTF-8 byte offsets for painting; platform text-event ranges are rune indices. Do not interchange them."}},
			{ID: "keyboard", Title: "Editing & limitations", Text: "The Web and native examples handle insertion, Backspace/Delete, Left/Right, Shift+Left/Right and composition through the same Go editor. macOS also delivers click placement, drag selection and Command-A.", Items: []string{"Single-line Tab changes focus; it does not insert a tab character.", "Read-only and disabled examples do not enable a platform editor.", "Browser and desktop bridges differ only in event transport; application text state remains shared Go state.", "Platform-specific clipboard shortcuts, undo/redo and multiline editing require separate backend verification."}},
		}},
		"code-diff": {Sections: []Section{
			{ID: "usage", Title: "Aligned source comparison", Text: "CodeDiff paints two synchronized panes from a caller-owned CodeDiffResult. The left pane is a fixed baseline; the right pane connects to the same native text editor used by CodeEditor and Textarea.", Items: []string{"Use DiffCodeLines to create Added, Removed and Changed rows without changing either source string.", "Long lines share one horizontal offset and both panes share one vertical offset.", "Added and removed rows include signs and line numbers in addition to color."}},
			{ID: "keyboard", Title: "Editing and scrolling", Text: "The right pane preserves byte-based selection, composition and caret placement while the aligned rows remain read-only on the left.", Items: []string{"Mouse wheel and scrollbar drags update the shared offsets.", "Read-only previews do not expose a platform editor.", "Applications should debounce expensive recomputation and discard stale results after newer text arrives."}},
		}},
		"split-button": {Sections: []Section{
			{ID: "usage", Title: "Two independent actions", Text: "SplitButton paints a joined primary action and menu trigger while declaring separate stable IDs. Call Clicked after Paint to route either action.", Items: []string{"The primary and menu halves have independent hover, focus, pressed and disabled semantics.", "Open is caller-owned and is exposed as expanded state on the menu trigger.", "Use SplitButton when the menu selects a variation of one primary outcome."}},
			{ID: "keyboard", Title: "Keyboard and accessibility", Text: "Tab reaches each non-disabled half separately; Enter and Space activate the focused half through the same hit path as a pointer.", Items: []string{"A menu-only trigger without visible text must set MenuLabel.", "Disabled controls remain visible and expose disabled semantics.", "The application owns menu contents, dismissal and focus restoration."}},
		}},
		"dialog": {Sections: []Section{
			{ID: "usage", Title: "A real modal scope", Text: "BeginDialog paints the overlay and leaves its layer and focus scope active. Paint the body, lay the footer buttons out with Actions (each is as wide as its label needs, right-aligned in the order given), then call End. The application owns the open boolean and every action ID.", Items: []string{"Use distinct stable IDs for the trigger, close, cancel and confirm actions.", "The scope traps keyboard traversal while open; dismissal restores prior focus when available.", "The example does not dismiss on outside click; use its buttons or Escape. Confirm and destructive cases start open."}},
			{ID: "keyboard", Title: "Open, confirm, dismiss", Text: "Open the example, Tab through its actions, press Escape to dismiss, or choose Confirm/Cancel. The result is visible in the preview; no external action is performed."},
		}},
	}
	for id, page := range overrides {
		pages[id] = page
	}
	for id, page := range pages {
		page.Capabilities = []Capability{
			{Platform: "macOS", Status: "Native preview", Detail: "AppKit + Metal. Run locally to check system fonts, focus and accessibility."},
			{Platform: "Windows", Status: "Experimental", Detail: "Native runner cross-builds; this documentation is not a Windows hardware-validation report."},
			{Platform: "Linux", Status: "Not available", Detail: "The current Linux backend cannot run these Canvas component examples."},
			{Platform: "Web", Status: "Canvas2D preview", Detail: "The same Go example compiled to WASM, not HTML/CSS imitation. Native performance and system integration are not measured here."},
		}
		if id == "input" {
			page.Capabilities[3] = Capability{Platform: "Web", Status: "Interactive text input", Detail: "Browser typing, selection and IME connect to the same caller-owned Go editor used by the native preview."}
		}
		page.Sections = append(page.Sections, Section{
			ID: "source-ownership", Title: "One source, two previews",
			Text:  "The code shown here is read from the compiled Go example file during the documentation build. It uses State from the same internal/componentdemo package; it is not a standalone main program. The native run command executes that complete package.",
			Items: []string{"Reset creates a new preview instance; it does not change production component source.", "The source fingerprint identifies Go build inputs, not a published release or a promise of platform parity.", "The Astro development server rebuilds Go previews when their source changes, then reloads the page. Failed builds leave the last successful preview intact. Published static sites require a new build and deployment."},
		})
		pages[id] = page
	}
	return pages
}
