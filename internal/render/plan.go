package render

import (
	"slices"

	"github.com/vivaui/viva/internal/scene"
)

// PlanKind is what one planned operation draws.
type PlanKind uint8

const (
	// PlanShapes draws Count instances starting at First in the frame's
	// rectangle list.
	PlanShapes PlanKind = iota + 1
	// PlanLabels draws the label at Label.
	PlanLabels
)

// LabelOrder is what a renderer needs to place a label among shape batches:
// the layer it composes in and how many shape instances the frame painted
// before it.
type LabelOrder struct {
	Layer        uint16
	ShapesBefore uint32
}

// PlanOp is one draw in paint order: a run of shape instances, or one label.
type PlanOp struct {
	Kind    PlanKind
	Layer   uint16
	Clipped bool
	Clip    scene.Rect
	First   int
	Count   int
	Label   int
}

// Plan interleaves shape batches and labels in paint order, splitting a batch
// when a label was painted inside it. Shape batches must already be ordered by
// layer, as GroupBatches returns them. Plan allocates the label order; callers
// that plan every frame use PlanWithOrder.
func Plan(dst []PlanOp, batches []Batch, labels []LabelOrder) []PlanOp {
	order := make([]int, len(labels))
	planned, _ := PlanWithOrder(dst, order, batches, labels)
	return planned
}

// PlanWithOrder is Plan with a reusable order scratch. It returns the grown
// order slice so a caller can keep it for the next frame.
func PlanWithOrder(dst []PlanOp, order []int, batches []Batch, labels []LabelOrder) ([]PlanOp, []int) {
	order = order[:0]
	for index := range labels {
		order = append(order, index)
	}
	slices.SortStableFunc(order, func(left, right int) int {
		if layers := int(labels[left].Layer) - int(labels[right].Layer); layers != 0 {
			return layers
		}
		return int(labels[left].ShapesBefore) - int(labels[right].ShapesBefore)
	})
	dst = dst[:0]
	for batchIndex, labelIndex := 0, 0; batchIndex < len(batches) || labelIndex < len(order); {
		layer := ^uint16(0)
		if batchIndex < len(batches) {
			layer = batches[batchIndex].Layer
		}
		if labelIndex < len(order) && labels[order[labelIndex]].Layer < layer {
			layer = labels[order[labelIndex]].Layer
		}
		batchEnd := batchIndex
		for batchEnd < len(batches) && batches[batchEnd].Layer == layer {
			batchEnd++
		}
		labelEnd := labelIndex
		for labelEnd < len(order) && labels[order[labelEnd]].Layer == layer {
			labelEnd++
		}
		cursor := 0
		if batchIndex < batchEnd {
			cursor = batches[batchIndex].First
		}
		for batchIndex < batchEnd || labelIndex < labelEnd {
			if labelIndex < labelEnd &&
				(batchIndex >= batchEnd || int(labels[order[labelIndex]].ShapesBefore) <= cursor) {
				dst = append(dst, PlanOp{Kind: PlanLabels, Layer: layer, Label: order[labelIndex]})
				labelIndex++
				continue
			}
			batch := batches[batchIndex]
			end := batch.First + batch.Count
			first := cursor
			for cursor < end && (labelIndex >= labelEnd || cursor < int(labels[order[labelIndex]].ShapesBefore)) {
				cursor++
			}
			if cursor > first {
				dst = append(dst, PlanOp{
					Kind: PlanShapes, Layer: layer,
					Clipped: batch.Clipped, Clip: batch.Clip,
					First: first, Count: cursor - first,
				})
			}
			if cursor == end {
				batchIndex++
				if batchIndex < batchEnd {
					cursor = batches[batchIndex].First
				}
			}
		}
	}
	return dst, order
}
