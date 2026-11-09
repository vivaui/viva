package ui

import (
	"context"
	"sync"

	"github.com/vivaui/viva/app"
)

// EffectState is one page's asynchronous work, keyed by a value: a load that
// belongs to the current directory, a request that belongs to a host. Create it
// once, with the rest of the model, and declare it in the page:
//
//	var load = ui.NewEffect[LoadResult, string](func(ctx context.Context) (LoadResult, error) {
//		return fetch(ctx)
//	})
//
//	func (m *model) page(c ui.C) {
//		ui.Effect(c, m.load, m.dir)
//		if result, err, ok := m.load.Take(); ok {
//			m.apply(result, err)
//		}
//	}
//
// The work runs once for a key. A different key cancels the work and starts the
// new one; a page that stops declaring the effect, or a window that closes,
// cancels it too. A result that arrives after its key changed, after the effect
// disappeared or after the window closed is dropped. The run function must
// return when ctx is done; the framework does not kill goroutines or poll.
//
// The run function must not touch a C, Box or Frame: it runs off the paint.
// Its result is read in the page with Take, on the UI thread, after the effect
// was declared; a page drawn into a canvas of the application's own without
// PageConfig.Invalidator keeps the result for the next paint but cannot wake the
// window by itself.
type EffectState[T any, K comparable] struct {
	run func(context.Context) (T, error)

	mu         sync.Mutex
	owner      *Page // the page the effect belongs to
	key        K
	keySet     bool
	running    bool
	listed     bool // in the page's effects
	runID      uint64
	cancelFn   context.CancelFunc
	result     T
	err        error
	has        bool
	epoch      uint32 // the paint that last declared it
	invalidate *app.Invalidator
}

// NewEffect makes the state of one effect. T is what the work returns and K is
// what names a run, such as a directory or a host ID.
func NewEffect[T any, K comparable](run func(context.Context) (T, error)) *EffectState[T, K] {
	if run == nil {
		panic("ui: NewEffect needs its run function")
	}
	return &EffectState[T, K]{run: run}
}

// Effect declares state's work for key in the page c. The same key does not
// start it again; a different key cancels the run and starts a new one. A state
// whose work finished and whose key did not change keeps its result for Take.
// Not inside a Memo, whose copies are not declared.
func Effect[T any, K comparable](c C, state *EffectState[T, K], key K) {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Effect is for a page")
	}
	if c.f.memoDepth > 0 {
		panic("ui: Effect in a Memo: a copied part is not declared, so its work would be cancelled; declare it outside the Memo")
	}
	if state == nil {
		panic("ui: Effect needs its state")
	}
	state.use(c, key)
}

// Take returns the result a run finished with, once, and clears it. It is read
// on the UI thread, after the effect was declared in the page.
func (s *EffectState[T, K]) Take() (T, error, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.has {
		var zero T
		return zero, nil, false
	}
	result, err := s.result, s.err
	var zero T
	s.result, s.err, s.has = zero, nil, false
	return result, err, true
}

func (s *EffectState[T, K]) use(c C, key K) {
	p := c.page
	if p.effectContext == nil {
		p.effectContext, p.effectCancel = context.WithCancel(context.Background())
	}
	s.mu.Lock()
	if p.effectContext.Err() != nil {
		s.mu.Unlock()
		return
	}
	if s.owner != nil && s.owner != p {
		s.mu.Unlock()
		panic("ui: an EffectState belongs to one page; create one for each page that declares it")
	}
	if s.owner == nil {
		s.owner = p
	}
	if s.epoch == p.epoch && s.keySet && s.key != key {
		s.mu.Unlock()
		panic("ui: an Effect was declared twice in one paint with different keys")
	}
	if s.epoch != p.epoch {
		s.epoch = p.epoch
		if !s.listed {
			s.listed = true
			p.effects = append(p.effects, s)
		}
	}
	if s.running {
		if s.keySet && s.key == key {
			s.mu.Unlock()
			return
		}
		s.cancelLocked()
	} else if s.keySet && s.key == key {
		s.mu.Unlock()
		return
	}
	s.key, s.keySet = key, true
	var zero T
	s.result, s.err, s.has = zero, nil, false
	s.invalidate = p.invalidator
	ctx, cancel := context.WithCancel(p.effectContext)
	s.cancelFn = cancel
	s.running = true
	s.runID++
	id, run := s.runID, s.run
	s.mu.Unlock()
	go func() {
		result, err := run(ctx)
		s.finish(id, result, err)
	}()
}

func (s *EffectState[T, K]) finish(id uint64, result T, err error) {
	s.mu.Lock()
	if s.runID != id {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.cancelFn = nil
	s.result, s.err, s.has = result, err, true
	invalidate := s.invalidate
	s.mu.Unlock()
	if invalidate != nil {
		invalidate.Invalidate()
	}
}

// cancel stops the run and forgets its key and result: the effect disappeared,
// or its window closed. A later declaration starts it again.
func (s *EffectState[T, K]) cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelLocked()
	s.keySet = false
	s.listed = false
	s.has = false
	s.invalidate = nil
	var zero T
	s.result, s.err = zero, nil
}

// cancelLocked invalidates the running work; its result, if it still comes,
// finds another runID and is dropped.
func (s *EffectState[T, K]) cancelLocked() {
	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
	}
	s.running = false
	s.runID++
}

func (s *EffectState[T, K]) seen() uint32 {
	s.mu.Lock()
	epoch := s.epoch
	s.mu.Unlock()
	return epoch
}

// effectState is an EffectState without its value and key types, so the page can
// cancel the ones it no longer declares.
type effectState interface {
	seen() uint32
	cancel()
}

// settleEffects cancels the effects the paint did not declare and keeps the
// page's list small.
func (p *Page) settleEffects() {
	if len(p.effects) == 0 {
		return
	}
	keep := p.effects[:0]
	for _, state := range p.effects {
		if state.seen() == p.epoch {
			keep = append(keep, state)
		} else {
			state.cancel()
		}
	}
	for i := len(keep); i < len(p.effects); i++ {
		p.effects[i] = nil // do not keep a cancelled effect's result alive
	}
	p.effects = keep
}

// Close ends the page's effects. Config's OnClosed calls it for a page that
// paints a window; a page the application draws itself calls it when the page
// goes away. It is safe to call more than once, and a closed page declares no
// new work.
func (p *Page) Close() {
	p.cancelEffects()
}

// cancelEffects stops every effect of the page and cancels the context their
// runs derive from, when the window closes.
func (p *Page) cancelEffects() {
	for _, state := range p.effects {
		state.cancel()
	}
	p.effects = p.effects[:0]
	if p.effectCancel != nil {
		p.effectCancel()
		p.effectCancel = nil
	}
}
