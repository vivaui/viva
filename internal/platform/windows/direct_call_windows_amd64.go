//go:build windows && amd64

package windows

// The x64 ABI passes this eight-byte aggregate in an integer argument slot.
type directTextCall struct{}

func (*directTarget) initTextCall() {}
func (target *directTarget) drawTextLayout(x, y float32, layout uintptr, options uint32) {
	comCall(target.target, d2dDrawTextLayout, pointArgument(x, y), layout, target.brush, uintptr(options))
}
