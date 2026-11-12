//go:build linux

package linux

import (
	"slices"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

type compositingBatchKind uint8

const (
	compositeRectangles compositingBatchKind = iota + 1
	compositeText
)

type compositingBatch struct {
	Kind         compositingBatchKind
	Layer        uint16
	Clipped      bool
	Clip         scene.Rect
	First, Count int
}

func groupTextLayers(labels []vulkanLabelTexture, order []int) []int {
	order = order[:0]
	for index := range labels {
		order = append(order, index)
	}
	slices.SortStableFunc(order, func(left, right int) int {
		return int(labels[left].label.Layer) - int(labels[right].label.Layer)
	})
	return order
}

// groupCompositingLayers merges the shape batches and the text labels of each
// layer into the order the application painted them. Each label says how many
// shapes were painted before it, so a label may fall between two instances of
// one batch, which then draws in two parts.
func groupCompositingLayers(rectangles []render.Batch, labels []vulkanLabelTexture, labelOrder []int, batches []compositingBatch) []compositingBatch {
	batches = batches[:0]
	for rectangleIndex, labelIndex := 0, 0; rectangleIndex < len(rectangles) || labelIndex < len(labelOrder); {
		layer := ^uint16(0)
		if rectangleIndex < len(rectangles) {
			layer = rectangles[rectangleIndex].Layer
		}
		if labelIndex < len(labelOrder) && labels[labelOrder[labelIndex]].label.Layer < layer {
			layer = labels[labelOrder[labelIndex]].label.Layer
		}
		batchEnd := rectangleIndex
		for batchEnd < len(rectangles) && rectangles[batchEnd].Layer == layer {
			batchEnd++
		}
		labelEnd := labelIndex
		for labelEnd < len(labelOrder) && labels[labelOrder[labelEnd]].label.Layer == layer {
			labelEnd++
		}
		cursor := 0
		if rectangleIndex < batchEnd {
			cursor = rectangles[rectangleIndex].First
		}
		for rectangleIndex < batchEnd || labelIndex < labelEnd {
			if labelIndex < labelEnd && (rectangleIndex >= batchEnd || int(labels[labelOrder[labelIndex]].label.ShapesBefore) <= cursor) {
				if count := len(batches); count > 0 && batches[count-1].Kind == compositeText &&
					batches[count-1].Layer == layer && batches[count-1].First+batches[count-1].Count == labelIndex {
					batches[count-1].Count++
				} else {
					batches = append(batches, compositingBatch{Kind: compositeText, Layer: layer, First: labelIndex, Count: 1})
				}
				labelIndex++
				continue
			}
			batch := rectangles[rectangleIndex]
			end := batch.First + batch.Count
			first := cursor
			for cursor < end && (labelIndex >= labelEnd || cursor < int(labels[labelOrder[labelIndex]].label.ShapesBefore)) {
				cursor++
			}
			if cursor > first {
				batches = append(batches, compositingBatch{Kind: compositeRectangles, Layer: layer, Clipped: batch.Clipped, Clip: batch.Clip, First: first, Count: cursor - first})
			}
			if cursor == end {
				rectangleIndex++
				if rectangleIndex < batchEnd {
					cursor = rectangles[rectangleIndex].First
				}
			}
		}
	}
	return batches
}

func validateInitialTextLabels(labels []platform.TextLabel) error {
	for index, label := range labels {
		if err := validateTextLabel(index, label); err != nil {
			return err
		}
	}
	return nil
}
