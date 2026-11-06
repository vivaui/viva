package platform

import "github.com/vivaui/viva/ui"

func TextAlignmentInset(alignment ui.Alignment, fieldWidth, lineWidth float32, terminal, placed bool) float32 {
	if terminal || placed || lineWidth >= fieldWidth {
		return 0
	}
	switch alignment {
	case ui.AlignCenter:
		return max(0, fieldWidth-lineWidth) / 2
	case ui.AlignRight:
		return max(0, fieldWidth-lineWidth)
	default:
		return 0
	}
}
