package gfx

import (
	"reflect"
	"testing"
)

// placement is a TextPlacement for the test, so Placement can be set.
type placement struct{}

func (placement) OffsetAt(Point) (int, bool) { return 0, false }
func (placement) CaretBounds() (Rect, bool)  { return Rect{}, false }

// setToOne makes a value of the type that is not its zero: a number at one,
// a flag on, a word, a filled struct, a pointer, a function or an interface
// with something in it.
func setToOne(value reflect.Value) {
	switch value.Kind() {
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(1)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(1)
	case reflect.Float32, reflect.Float64:
		value.SetFloat(1)
	case reflect.String:
		value.SetString("set")
	case reflect.Struct:
		setToOne(value.Field(0))
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
	default:
		panic("the test has no value for a " + value.Type().String())
	}
}

// A struct with a function in it cannot be compared with the zero value, so
// HitOptions says itself whether anything was set. That must notice a field
// as soon as one is added: a hit with only that field set is not the zero
// options, wherever HitWith was handed them.
func TestHitOptionsIsZeroSeesEveryField(t *testing.T) {
	if !(HitOptions{}).IsZero() {
		t.Fatal("the zero options report as set")
	}
	typ := reflect.TypeOf(HitOptions{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		options := HitOptions{}
		if !field.IsExported() {
			// reflect cannot set an unexported field; the test is in the package.
			options.label = true
			if options.IsZero() {
				t.Fatal("only label set reads as the zero options")
			}
			continue
		}
		one := reflect.ValueOf(&options).Elem().Field(i)
		switch field.Name {
		case "CursorImageAt":
			one.Set(reflect.ValueOf(CursorImageFunc(func(Point) *CursorImage { return nil })))
		case "Placement":
			one.Set(reflect.ValueOf(placement{}))
		default:
			setToOne(one)
		}
		if options.IsZero() {
			t.Fatalf("only %s set reads as the zero options", field.Name)
		}
	}
}
