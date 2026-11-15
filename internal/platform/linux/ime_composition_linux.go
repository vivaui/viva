package linux

import (
	"errors"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// The optional native session survives empty and hidden preedit. Its mode is
// meaningful only after ClientCommitPreedit was accepted by this context.
type ibusComposition struct {
	native   ibus.PreeditEvent
	revision uint64
	session  bool
}

func (ime *linuxIME) bindOwner() {
	ime.target, ime.owner = ime.text.input.Target, ime.text.state.CompositionIdentity
	ime.tentative = ime.text.state.TentativeComposition
	ime.phase = nil
	if ime.tentative {
		ime.phase = &ibusComposition{revision: ime.text.stateRevision}
	}
}

func (ime *linuxIME) sameOwner() bool {
	return ime.text.input.Target == ime.target && ime.text.state.CompositionIdentity == ime.owner &&
		ime.text.state.TentativeComposition == ime.tentative
}

func (ime *linuxIME) current() bool {
	return !ime.dead && ime.context != nil && !ime.text.closed && ime.text.enabled &&
		ime.app.firstErr == nil && !ime.app.quit && ime.sameOwner()
}

func (ime *linuxIME) keyCurrent(epoch uint64) bool {
	return ime.epoch == epoch && !ime.text.closed && ime.text.enabled &&
		(ime.phase == nil || ime.active || ime.dead) && ime.app.firstErr == nil && !ime.app.quit && ime.sameOwner()
}

func (ime *linuxIME) open() bool {
	bus := ime.app.inputMethodBus()
	if bus == nil {
		return false
	}
	ime.bindOwner()
	ime.epoch++
	epoch := ime.epoch
	valid := func() bool { return ime.epoch == epoch && ime.current() }
	context, err := bus.NewContext("viva", ibus.Handler{
		Commit: func(value string) {
			if valid() {
				ime.commit(value)
			}
		},
		Preedit: func(event ibus.PreeditEvent) {
			if valid() {
				ime.nativePreedit(event)
			}
		},
		Forward: func(symbol, code, state uint32) {
			if valid() {
				ime.forward(symbol, code, state)
			}
		},
		DeleteSurrounding: func(offset, count int) {
			if valid() {
				ime.deleteSurrounding(offset, count)
			}
		},
	})
	if err != nil {
		ime.app.checkInputMethod()
		return false
	}
	ime.context = context
	ime.negotiateTentative(context)
	return true
}

// negotiateTentative asks the context for native preedit modes. It reports
// whether explicit phases are available. A refusal is not a failure: the
// field keeps its input-method input through legacy marked-text events, so
// the context stays and only the optional phase protocol is dropped.
func (ime *linuxIME) negotiateTentative(context interface{ SetClientCommitPreedit(bool) error }) bool {
	if !ime.tentative {
		return true
	}
	if err := context.SetClientCommitPreedit(true); err != nil {
		ime.phase = nil
		return false
	}
	return true
}

func (ime *linuxIME) retire() {
	context := ime.context
	ime.epoch++
	ime.context, ime.phase = nil, nil
	ime.pendingKeys = 0
	ime.active, ime.engineComposing = false, false
	ime.haveCursor, ime.hasSources, ime.sent = false, false, false
	if context != nil {
		context.Close()
	}
}

func (ime *linuxIME) clearComposition() {
	if ime.phase != nil {
		ime.phase.session = false
		ime.phase.native = ibus.PreeditEvent{}
	}
}

func (ime *linuxIME) composing() bool {
	return ime.text.state.Composing || ime.phase != nil && ime.phase.session
}

func (ime *linuxIME) nativePreedit(event ibus.PreeditEvent) {
	if !ime.current() || ime.phase != nil && !ime.active {
		return
	}
	if ime.phase == nil {
		ime.preedit(event.Text, event.Cursor, event.Visible, event.Attributes)
		return
	}
	phase := ime.phase
	switch event.Kind {
	case ibus.PreeditUpdate:
		phase.native = event
		if !phase.session && (!event.Visible || event.Text == "") {
			return
		}
		phase.session = true
	case ibus.PreeditHide, ibus.PreeditShow:
		if !phase.session {
			return
		}
		phase.native.Visible = event.Kind == ibus.PreeditShow
	}
	value, selection := "", text.Range{}
	if phase.native.Visible {
		value = phase.native.Text
		length := utf8.RuneCountInString(value)
		selection = preeditSelection(length, min(max(phase.native.Cursor, 0), length), phase.native.Attributes)
	}
	ime.engineComposing = value != ""
	ime.apply(platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: selection, CompositionPhase: text.CompositionUpdate})
}

func (ime *linuxIME) reset() error {
	if ime.app.firstErr != nil {
		return ime.app.firstErr
	}
	if ime.dead || ime.context == nil || ime.text.closed || ime.app.quit || !ime.sameOwner() {
		return ibus.ErrClosed
	}
	context, epoch := ime.context, ime.epoch
	if ime.pendingKeys != 0 {
		return errors.New("linux: input method keys are still pending")
	}
	event := platform.TextEvent{}
	settle := ime.phase != nil && ime.phase.session && ime.sameOwner()
	if settle {
		native := ime.phase.native
		if !native.HasMode || native.Mode != ibus.PreeditModeClear && native.Mode != ibus.PreeditModeCommit {
			return errors.New("linux: input method supplied no supported preedit mode")
		}
		event = platform.TextEvent{Kind: platform.TextCancelComposition, CompositionPhase: text.CompositionCancel}
		if native.Visible && native.Mode == ibus.PreeditModeCommit {
			event = platform.TextEvent{Kind: platform.TextInsert, CompositionPhase: text.CompositionResult, Text: native.Text}
		}
	}
	if err := context.Reset(); err != nil {
		return err
	}
	if ime.context != context || ime.epoch != epoch || !ime.sameOwner() {
		return nil
	}
	ime.engineComposing = false
	ime.clearComposition()
	if settle {
		ime.apply(event)
	}
	return nil
}

func (ime *linuxIME) sendKey(symbol, code, state uint32) error {
	epoch := ime.epoch
	ime.pendingKeys++
	err := ime.context.ProcessKey(symbol, code, state, func(bool) {
		if ime.answerKey(epoch) {
			ime.finishKey(epoch)
		}
	})
	if err != nil && ime.epoch == epoch {
		ime.pendingKeys--
	}
	return err
}

func (ime *linuxIME) answerKey(epoch uint64) bool {
	if ime.epoch != epoch {
		return false
	}
	ime.pendingKeys--
	if ime.app.ibus != nil && ime.app.ibus.Err() != nil {
		ime.app.checkInputMethod()
		if !ime.dead {
			ime.lost()
		}
	}
	return ime.epoch == epoch
}

func (ime *linuxIME) finishKey(epoch uint64) {
	if ime.epoch == epoch && ime.pendingKeys == 0 {
		ime.sync()
	}
}
