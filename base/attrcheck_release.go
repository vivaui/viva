//go:build !dev && !vivadev

package base

// checkAttributes is empty in release builds: the attribute rules and their
// messages exist only in the dev build (attrcheck_dev.go).
func (f *Frame) checkAttributes() {}

// markAttr is empty in release builds: no attribute set is recorded.
func (n *Node) markAttr(uint8) {}

// unusedName is empty in release builds: a Key or Part given to an element
// without identity is dropped, not reported.
func (f *Frame) unusedName(int32, protoName) {}
