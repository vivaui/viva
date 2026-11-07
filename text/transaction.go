package text

import (
	"fmt"
	"slices"
)

// Transact edits an isolated draft and commits its text, selection and actual
// primitives as one existing history unit. The draft has no source Undo/Redo.
// The callback must only edit the draft
// through its methods; it must not copy/replace it or mutate the source. Editors
// remain single-threaded. Callback errors leave the source unchanged. Temporary
// storage grows with staged edits; the source keeps its usual history/journal limits.
func (editor *Editor) Transact(apply func(*Editor) error) error {
	if editor == nil || apply == nil {
		return fmt.Errorf("transaction source and callback are required")
	}
	if editor.composing {
		return fmt.Errorf("cannot transact during composition")
	}
	if editor.accept != nil || editor.maxRunes > 0 {
		return fmt.Errorf("cannot transact in a restricted editor")
	}
	before, selection := editor.String(), editor.selectionState()
	draft := NewEditor(before)
	draft.anchor, draft.caret = selection.anchor, selection.caret
	draft.code, draft.codeSet, draft.overwrite = editor.code, editor.codeSet, editor.overwrite
	recorder := &editJournal{unlimited: true}
	draft.journal = recorder
	defer func() { draft.journal = nil }()
	if err := apply(draft); err != nil {
		return err
	}
	if draft.journal != recorder || draft.composing {
		return fmt.Errorf("transaction draft lost its recorder or has unfinished composition")
	}
	if editor.String() != before || editor.selectionState() != selection {
		return fmt.Errorf("transaction source changed during callback")
	}
	value := draft.String()
	after := Range{Start: draft.anchor, End: draft.caret}
	target := Range{End: len(editor.runes)}
	if err := editor.ValidateReplaceRanges([]Replacement{{Range: target, Text: value}}, ReplaceRangesOptions{AfterSelection: &after}); err != nil {
		return err
	}
	primitives := slices.Clone(recorder.entries)
	if len(primitives) == 0 && value == before {
		editor.SetSelection(after)
		return nil
	}
	editor.commitBatch(target, value, primitives, &after, value != before)
	return nil
}
