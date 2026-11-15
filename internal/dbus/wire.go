package dbus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Encoder writes values in the D-Bus wire format. Alignment is relative to the
// start of the buffer, so a message body, which starts on an 8-byte boundary, is
// encoded in a buffer of its own.
type Encoder struct{ buf []byte }

// Bytes is what has been written.
func (e *Encoder) Bytes() []byte { return e.buf }

// Align pads with zeros to a multiple of n.
func (e *Encoder) Align(n int) {
	for len(e.buf)%n != 0 {
		e.buf = append(e.buf, 0)
	}
}

func (e *Encoder) Byte(value byte) { e.buf = append(e.buf, value) }

func (e *Encoder) Bool(value bool) {
	if value {
		e.Uint32(1)
	} else {
		e.Uint32(0)
	}
}

func (e *Encoder) Int32(value int32) { e.Uint32(uint32(value)) }

func (e *Encoder) Uint32(value uint32) {
	e.Align(4)
	e.buf = binary.LittleEndian.AppendUint32(e.buf, value)
}

// Float64 writes a double, which the wire format aligns on eight bytes.
func (e *Encoder) Float64(value float64) {
	e.Align(8)
	e.buf = binary.LittleEndian.AppendUint64(e.buf, math.Float64bits(value))
}

// String writes a UTF-8 string; an object path has the same encoding.
func (e *Encoder) String(value string) {
	e.Uint32(uint32(len(value)))
	e.buf = append(e.buf, value...)
	e.buf = append(e.buf, 0)
}

// Signature writes a type signature, whose length is a single byte.
func (e *Encoder) Signature(value string) {
	e.Byte(byte(len(value)))
	e.buf = append(e.buf, value...)
	e.buf = append(e.buf, 0)
}

// Variant starts a variant of signature sig; the caller writes the value next.
func (e *Encoder) Variant(sig string) { e.Signature(sig) }

// ArrayMark remembers where an array began.
type ArrayMark struct{ length, data int }

// BeginArray starts an array whose elements need the alignment elem.
func (e *Encoder) BeginArray(elem int) ArrayMark {
	e.Uint32(0)
	length := len(e.buf) - 4
	e.Align(elem)
	return ArrayMark{length: length, data: len(e.buf)}
}

// EndArray closes the array begun at mark, filling in its byte length.
func (e *Encoder) EndArray(mark ArrayMark) {
	binary.LittleEndian.PutUint32(e.buf[mark.length:], uint32(len(e.buf)-mark.data))
}

// ErrShort means a message ended before a value did.
var ErrShort = errors.New("dbus: message is shorter than its contents")

// Decoder reads values in the D-Bus wire format. The first error sticks and
// every later read returns zero values.
type Decoder struct {
	buf []byte
	pos int
	err error
}

// NewDecoder reads buf from its start.
func NewDecoder(buf []byte) *Decoder { return &Decoder{buf: buf} }

// Err is the first error met.
func (d *Decoder) Err() error { return d.err }

// Done reports whether every byte has been read.
func (d *Decoder) Done() bool { return d.pos >= len(d.buf) }

func (d *Decoder) Align(n int) {
	for d.pos%n != 0 {
		d.pos++
	}
	if d.pos > len(d.buf) && d.err == nil {
		d.err = ErrShort
	}
}

func (d *Decoder) take(n int) []byte {
	if d.err != nil || n < 0 || d.pos+n > len(d.buf) {
		if d.err == nil {
			d.err = ErrShort
		}
		return nil
	}
	out := d.buf[d.pos : d.pos+n]
	d.pos += n
	return out
}

func (d *Decoder) Byte() byte {
	if b := d.take(1); b != nil {
		return b[0]
	}
	return 0
}

func (d *Decoder) Uint32() uint32 {
	d.Align(4)
	if b := d.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

func (d *Decoder) Int32() int32 { return int32(d.Uint32()) }

func (d *Decoder) Bool() bool { return d.Uint32() != 0 }

// Float64 reads a double.
func (d *Decoder) Float64() float64 {
	d.Align(8)
	if b := d.take(8); b != nil {
		return math.Float64frombits(binary.LittleEndian.Uint64(b))
	}
	return 0
}

// String reads a string or an object path.
func (d *Decoder) String() string {
	n := int(d.Uint32())
	b := d.take(n)
	d.take(1) // the terminating zero
	return string(b)
}

func (d *Decoder) Signature() string {
	n := int(d.Byte())
	b := d.take(n)
	d.take(1)
	return string(b)
}

// UnixFD reads a unix file descriptor index, which names an entry of the
// message's FDs.
func (d *Decoder) UnixFD() int { return int(d.Uint32()) }

// ArrayLength reads an array's byte length and aligns to its elements, and
// returns the offset where they end.
func (d *Decoder) ArrayLength(elem int) (end int) {
	n := int(d.Uint32())
	d.Align(elem)
	end = d.pos + n
	if end > len(d.buf) && d.err == nil {
		d.err = ErrShort
	}
	return end
}

// Pos is the offset of the next byte; an array loop runs while Pos < end.
func (d *Decoder) Pos() int { return d.pos }

// Skip steps over one value of the single complete type sig, which may be a
// container. It is how a handler ignores arguments it has no use for.
func (d *Decoder) Skip(sig string) {
	rest, err := d.skip(sig)
	if err == nil && rest != "" {
		err = fmt.Errorf("dbus: signature %q has more than one type", sig)
	}
	if err != nil && d.err == nil {
		d.err = err
	}
}

// SkipAll steps over every value of a body signature.
func (d *Decoder) SkipAll(sig string) {
	for sig != "" && d.err == nil {
		var err error
		if sig, err = d.skip(sig); err != nil {
			d.err = err
		}
	}
}

func (d *Decoder) skip(sig string) (rest string, err error) {
	if sig == "" {
		return "", errors.New("dbus: empty type")
	}
	switch sig[0] {
	case 'y':
		d.take(1)
	case 'b', 'i', 'u':
		d.Uint32()
	case 'n', 'q':
		d.Align(2)
		d.take(2)
	case 'x', 't', 'd':
		d.Align(8)
		d.take(8)
	case 's', 'o':
		_ = d.String()
	case 'g':
		_ = d.Signature()
	case 'h':
		_ = d.UnixFD()
	case 'v':
		d.Skip(d.Signature())
	case 'a':
		element, err := completeType(sig[1:])
		if err != nil {
			return "", err
		}
		end := d.ArrayLength(alignment(element[0]))
		for d.err == nil && d.pos < end {
			if _, err := d.skip(element); err != nil {
				return "", err
			}
		}
		return sig[1+len(element):], d.err
	case '(', '{':
		close := byte(')')
		if sig[0] == '{' {
			close = '}'
		}
		d.Align(8)
		inner := sig[1:]
		if len(inner) > 0 && inner[0] == close {
			// An empty container takes no bytes, so an array of them would never end.
			return "", fmt.Errorf("dbus: empty container in %q", sig)
		}
		for len(inner) > 0 && inner[0] != close {
			var err error
			if inner, err = d.skip(inner); err != nil {
				return "", err
			}
		}
		if len(inner) == 0 {
			return "", fmt.Errorf("dbus: unterminated container in %q", sig)
		}
		return inner[1:], d.err
	default:
		return "", fmt.Errorf("dbus: unknown type %q", sig[0])
	}
	return sig[1:], d.err
}

// completeType is the first complete type at the start of sig.
func completeType(sig string) (string, error) {
	if sig == "" {
		return "", errors.New("dbus: missing array element type")
	}
	switch sig[0] {
	case 'a':
		inner, err := completeType(sig[1:])
		if err != nil {
			return "", err
		}
		return sig[:1+len(inner)], nil
	case '(', '{':
		close, depth := byte(')'), 0
		if sig[0] == '{' {
			close = '}'
		}
		if len(sig) > 1 && sig[1] == close {
			return "", fmt.Errorf("dbus: empty container in %q", sig)
		}
		for index := 0; index < len(sig); index++ {
			switch sig[index] {
			case '(', '{':
				depth++
			case ')', '}':
				depth--
				if depth == 0 {
					if sig[index] != close {
						return "", fmt.Errorf("dbus: mismatched container in %q", sig)
					}
					return sig[:index+1], nil
				}
			}
		}
		return "", fmt.Errorf("dbus: unterminated container in %q", sig)
	}
	return sig[:1], nil
}

func alignment(kind byte) int {
	switch kind {
	case 'n', 'q':
		return 2
	case 'b', 'i', 'u', 's', 'o', 'a', 'h':
		return 4
	case 'x', 't', 'd', '(', '{':
		return 8
	}
	return 1
}
