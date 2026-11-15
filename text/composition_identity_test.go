package text

import "testing"

func TestTentativeCompositionActiveExcludesLegacyAndForeignOwners(t *testing.T) {
	var absent *Editor
	editor := NewEditor("abc")
	if absent.TentativeCompositionActive() || editor.TentativeCompositionActive() || editor.journal != nil {
		t.Fatal("inactive read allocated state or reported rollback ownership")
	}
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	editor.SetMarkedText("legacy", Range{})
	if !editor.CompositionActive() || editor.TentativeCompositionActive() {
		t.Fatal("legacy marking was mistaken for explicit rollback ownership")
	}
	editor.Unmark()
	if !editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: "p"}) || !editor.TentativeCompositionActive() {
		t.Fatal("explicit Update did not expose its live rollback boundary")
	}
	foreign := *editor
	if foreign.TentativeCompositionActive() {
		t.Fatal("a copied receiver borrowed another editor's rollback boundary")
	}
	if allocations := testing.AllocsPerRun(100, func() {
		if !editor.TentativeCompositionActive() {
			t.Fatal("read changed live rollback ownership")
		}
	}); allocations != 0 {
		t.Fatalf("active reads allocated %.0f times", allocations)
	}
	if !editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate}) || editor.TentativeCompositionActive() || editor.CompositionActive() {
		t.Fatal("empty Update did not end the explicit rollback boundary")
	}
}

func TestCompositionIdentityTracksOnlyAnEnabledCapability(t *testing.T) {
	var absent *Editor
	first, second := NewEditor("same"), NewEditor("same")
	if absent.TentativeCompositionIdentity() != (CompositionIdentity{}) || first.TentativeCompositionIdentity() != (CompositionIdentity{}) || first.journal != nil {
		t.Fatal("reading default identity enabled or retained optional state")
	}
	if !first.EnableTentativeComposition() || !second.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	identity := first.TentativeCompositionIdentity()
	if identity == (CompositionIdentity{}) || identity == second.TentativeCompositionIdentity() {
		t.Fatal("different enabled capabilities have no distinct identities")
	}
	if !first.EnableTentativeComposition() || first.TentativeCompositionIdentity() != identity {
		t.Fatal("an already-enabled capability changed identity")
	}
	if !first.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: "x"}) {
		t.Fatal("composition update refused")
	}
	before, selection := first.String(), first.Selection()
	if allocations := testing.AllocsPerRun(100, func() {
		if first.TentativeCompositionIdentity() != identity {
			t.Fatal("an active snapshot changed ownership")
		}
	}); allocations != 0 {
		t.Fatalf("identity snapshots allocate: %v", allocations)
	}
	if first.String() != before || first.Selection() != selection || !first.CompositionActive() {
		t.Fatal("identity lookup changed the live composition")
	}
	foreign := *first
	if foreign.TentativeCompositionIdentity() != (CompositionIdentity{}) || foreign.TentativeCompositionEnabled() {
		t.Fatal("a value copy borrowed its source's capability ownership")
	}
	if !first.ApplyComposition(CompositionEvent{Phase: CompositionCancel}) {
		t.Fatal("composition cancel refused")
	}
	first.ClearHistory()
	if first.TentativeCompositionIdentity() != identity {
		t.Fatal("clearing history changed the enabled editor's ownership")
	}
	if !first.DisableTentativeComposition() || first.TentativeCompositionIdentity() != (CompositionIdentity{}) {
		t.Fatal("disabled capability retained a visible identity")
	}
	if !first.EnableTentativeComposition() || first.TentativeCompositionIdentity() == identity || first.TentativeCompositionIdentity() == (CompositionIdentity{}) {
		t.Fatal("re-enabling a released capability reused stale ownership")
	}
}

func TestCompositionIdentityRefreshesWhenPassiveStorageIsReenabled(t *testing.T) {
	editor := NewEditor("abc")
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	identity := editor.TentativeCompositionIdentity()
	if _, _, ok := editor.TrackHistory(); !ok {
		t.Fatal("history setup refused")
	}
	if !editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: "x"}) ||
		!editor.ApplyComposition(CompositionEvent{Phase: CompositionResult, Text: "z"}) {
		t.Fatal("composition result setup refused")
	}
	state := editor.compositionState()
	if state == nil || !state.completed || !editor.DisableTentativeComposition() || !state.passive {
		t.Fatal("completed action did not retain passive storage")
	}
	if editor.TentativeCompositionIdentity() != (CompositionIdentity{}) {
		t.Fatal("passive storage reported enabled ownership")
	}
	before, selection := editor.String(), editor.Selection()
	undo, redo := editor.CanUndo(), editor.CanRedo()
	cursor, _, ok := editor.TrackHistory()
	if !ok {
		t.Fatal("history cursor setup refused")
	}
	_, cursor, ok = editor.HistorySince(cursor)
	if !ok {
		t.Fatal("history prefix was unavailable before re-enable")
	}
	if !editor.EnableTentativeComposition() || editor.compositionState() != state {
		t.Fatal("re-enable did not reuse the existing charged storage")
	}
	if got := editor.TentativeCompositionIdentity(); got == identity || got == (CompositionIdentity{}) {
		t.Fatal("re-enable resurrected the retired native capability identity")
	}
	if editor.String() != before || editor.Selection() != selection || editor.CanUndo() != undo || editor.CanRedo() != redo {
		t.Fatal("identity refresh changed editing or history state")
	}
	if frames, latest, ok := editor.HistorySince(cursor); !ok || len(frames) != 0 || latest != cursor {
		t.Fatal("identity refresh published or invalidated history metadata")
	}
}
