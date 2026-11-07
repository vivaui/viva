//go:build js && wasm

package app

import (
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/web"
)

// webTextInput adapts the application's TextInputProvider to the browser's.
// The input it hands the runtime is reused, so resolving the focused field on a
// repaint allocates nothing beyond the editor's own snapshot.
func webTextInput(provider TextInputProvider) web.TextInputProvider {
	if provider == nil {
		return nil
	}
	var active *TextInput
	native := platform.TextInput{
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			if active == nil {
				return platform.TextState{}, nil
			}
			if active.Editor != nil {
				event.Apply(active.Editor)
				return platform.EditorState(active.Editor), nil
			}
			if active.OnEvent == nil {
				return platformTextState(active.State), nil
			}
			state, err := active.OnEvent(appTextEvent(event))
			return platformTextState(state), err
		},
	}
	return func(context web.TextInputContext) *platform.TextInput {
		active = provider(TextInputContext{
			ID: context.ID, Bounds: context.Bounds, Multiline: context.Multiline,
			Layer: context.Layer, ReadOnly: context.ReadOnly, Secure: context.Secure,
			NavigationKeys: context.NavigationKeys, Terminal: context.Terminal, TypeAhead: context.TypeAhead,
			TextBounds: context.TextBounds, FontSize: context.FontSize, FontWeight: context.FontWeight,
			TextColor: context.TextColor, HideNativeOverlays: context.HideNativeOverlays,
		})
		if active == nil {
			return nil
		}
		native.Target, native.Multiline = context.ID, context.Multiline
		native.Layer = context.Layer
		native.Secure = context.Secure
		native.Terminal = context.Terminal || context.TypeAhead
		native.FontSize, native.Weight = active.FontSize, active.Weight
		native.MonoFace = active.MonoFace
		native.AltSendsEscape = active.AltSendsEscape
		native.KeyReleases = active.KeyReleases && context.Terminal
		native.NavigationKeys = active.NavigationKeys || context.NavigationKeys
		native.HideNativeOverlays = active.HideNativeOverlays || context.HideNativeOverlays
		native.X, native.Y = active.Bounds.X, active.Bounds.Y
		native.Width, native.Height = active.Bounds.Width, active.Bounds.Height
		native.Color = platform.Color{
			Red: float64(active.Color.Red), Green: float64(active.Color.Green),
			Blue: float64(active.Color.Blue), Alpha: float64(active.Color.Alpha),
		}
		if active.Editor != nil {
			native.State = platform.EditorState(active.Editor)
		} else {
			native.State = platformTextState(active.State)
		}
		native.Alignment = resolveTextInputAlignment(active.Alignment, native.State.Value, native.State.Selection.End)
		return &native
	}
}

func platformTextState(state TextState) platform.TextState {
	return platform.TextState{
		Value:             state.Value,
		Selection:         platformTextRange(state.Selection),
		SelectionReversed: state.SelectionReversed,
		Composition:       platformTextRange(state.Composition),
		Composing:         state.Composing,
		Redraw:            state.Redraw,
	}
}

func appTextEvent(event platform.TextEvent) TextEvent {
	return TextEvent{
		Kind: TextEventKind(event.Kind), Text: event.Text,
		Selection:       appTextRange(event.Selection),
		MarkedSelection: appTextRange(event.MarkedSelection),
		Replacement:     appTextRange(event.Replacement), HasReplacement: event.HasReplacement,
		Command: TextCommand(event.Command),
	}
}
