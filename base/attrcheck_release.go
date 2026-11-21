//go:build !dev && !vivadev

package base

// checkAttributes is empty in release builds: the attribute rules and their
// messages exist only in the dev build (attrcheck_dev.go).
func (f *Frame) checkAttributes() {}

// markAttr is empty in release builds: no attribute set is recorded.
func (n *Node) markAttr(uint8) {}
