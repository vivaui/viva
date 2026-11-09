package textenginetest

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The reference metrics are the engine tests and tools install; they must keep the same contract.
func TestReferenceEnginePassesTheContract(t *testing.T) {
	Run(t, gfx.ReferenceTextEngine())
}
