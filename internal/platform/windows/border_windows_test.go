package windows

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestColorRefPacksWindowsChannelOrder(t *testing.T) {
	color := platform.Color{Red: 0.1, Green: 0.2, Blue: 0.3, Alpha: 1}
	if got, want := colorRef(color), uint32(0x004D331A); got != want {
		t.Fatalf("colorRef(%v) = %#08x, want %#08x", color, got, want)
	}
	clamped := platform.Color{Red: 2, Green: -1, Blue: 0.5, Alpha: 1}
	if got, want := colorRef(clamped), uint32(0x008000FF); got != want {
		t.Fatalf("colorRef(%v) = %#08x, want %#08x", clamped, got, want)
	}
}

func TestWindowBorderColorFollowsTheWindowEdge(t *testing.T) {
	clear := platform.Color{Red: 0.94, Green: 0.94, Blue: 0.95, Alpha: 1}
	tests := []struct {
		name        string
		config      platform.Config
		custom      bool
		backdrop    bool
		wantColor   uint32
		wantColored bool
	}{
		{
			name:        "custom titlebar uses the clear color",
			config:      platform.Config{ClearColor: clear},
			custom:      true,
			wantColor:   colorRef(clear),
			wantColored: true,
		},
		{
			name: "the application's color wins over the clear color",
			config: platform.Config{
				ClearColor:        clear,
				WindowBorderColor: func() platform.Color { return platform.Color{Red: 1, Green: 0, Blue: 0, Alpha: 1} },
			},
			custom:      true,
			wantColor:   0x000000FF,
			wantColored: true,
		},
		{
			name:      "a material keeps the system border",
			config:    platform.Config{ClearColor: clear},
			custom:    true,
			backdrop:  true,
			wantColor: dwmColorDefault,
		},
		{
			name:      "a transparent window keeps the system border",
			config:    platform.Config{ClearColor: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 0.5}},
			custom:    true,
			wantColor: dwmColorDefault,
		},
		{
			name:      "a system frame keeps its own border",
			config:    platform.Config{ClearColor: clear},
			wantColor: dwmColorDefault,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			color, colored := windowBorderColor(test.config, test.custom, test.backdrop)
			if color != test.wantColor || colored != test.wantColored {
				t.Fatalf("windowBorderColor = %#08x, %v; want %#08x, %v", color, colored, test.wantColor, test.wantColored)
			}
		})
	}
}
