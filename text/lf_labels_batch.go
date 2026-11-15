package text

import (
	"fmt"
	"slices"
)

type lfLabelSpan struct {
	line              int
	removed, inserted []rune
}

type lfLabelPlan struct {
	staged  *lfLabelState
	changed bool
}

func cloneLFLabels(source *lfLabelState, owner *Editor) *lfLabelState {
	return &lfLabelState{
		ops:          source.ops,
		owner:        owner,
		current:      slices.Clone(source.current),
		defaultLabel: source.defaultLabel,
		mapper:       source.mapper,
		staging:      true,
	}
}

func planLFLabelBatch(source *lfLabelState, spans []lfLabelSpan) (*lfLabelPlan, error) {
	if source.mapping {
		panic("text: LF label mapper reentered a batch")
	}
	before, selection, revision := source.owner.String(), source.owner.selectionState(), source.revision
	secondary := slices.Clone(source.owner.secondary)
	staged := cloneLFLabels(source, source.owner)
	source.mapping = true
	defer func() { source.mapping = false }()
	for _, span := range spans {
		replaceLFLabels(staged, span.line, span.removed, span.inserted)
	}
	if source.owner.String() != before || source.owner.selectionState() != selection || !slices.Equal(source.owner.secondary, secondary) || source.revision != revision {
		return nil, fmt.Errorf("LF label mapper changed the source during a batch")
	}
	return finishLFLabelPlan(source, staged), nil
}

func finishLFLabelPlan(source, staged *lfLabelState) *lfLabelPlan {
	if staged == nil || len(staged.events) == 0 {
		return nil
	}
	changed := len(source.current) != len(staged.current)
	if !changed {
		for index, cell := range source.current {
			if cell.value != staged.current[index].value {
				changed = true
				break
			}
		}
	}
	return &lfLabelPlan{staged: staged, changed: changed}
}

func commitLFLabelPlan(source *lfLabelState, plan *lfLabelPlan) {
	source.current = plan.staged.current
	source.pending = append(source.pending, plan.staged.events...)
	if source.staging {
		source.events = append(source.events, plan.staged.events...)
	}
	source.revision++
}
