package dbus

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Message types.
const (
	TypeCall   = 1
	TypeReturn = 2
	TypeError  = 3
	TypeSignal = 4
)

// Flags of a message.
const (
	flagNoReply = 1
)

// Header field codes.
const (
	fieldPath        = 1
	fieldInterface   = 2
	fieldMember      = 3
	fieldErrorName   = 4
	fieldReplySerial = 5
	fieldDestination = 6
	fieldSender      = 7
	fieldSignature   = 8
	fieldUnixFDs     = 9
)

const maxMessage = 1 << 27 // the specification's limit, 128 MiB

// Message is one D-Bus message. Body holds the encoded arguments, whose types
// Signature names.
type Message struct {
	Type        byte
	Flags       byte
	Serial      uint32
	ReplySerial uint32
	Path        string
	Interface   string
	Member      string
	ErrorName   string
	Destination string
	Sender      string
	Signature   string
	Body        []byte
	// FDs are the file descriptors the message carried, in the order its unix
	// fd indices name; a body type 'h' is an index into this slice. The side
	// that receives a message owns its descriptors and closes them: pass them
	// to TakeFD and close what TakeFD does not hand out, or call CloseFDs.
	FDs []int
	// unixFDCount is the header's count of descriptors; fill matches it with
	// the descriptors the socket delivered.
	unixFDCount int
}

// NoReply reports whether the sender wants no answer to a call.
func (m *Message) NoReply() bool { return m.Flags&flagNoReply != 0 }

// Args is a decoder over the message's arguments.
func (m *Message) Args() *Decoder { return NewDecoder(m.Body) }

// encode returns the message as it goes on the wire.
func (m *Message) encode() []byte {
	var e Encoder
	e.Byte('l')
	e.Byte(m.Type)
	e.Byte(m.Flags)
	e.Byte(1)
	e.Uint32(uint32(len(m.Body)))
	e.Uint32(m.Serial)
	start := e.BeginArray(8)
	field := func(code byte, sig string, write func()) {
		e.Align(8)
		e.Byte(code)
		e.Signature(sig)
		write()
	}
	if m.Path != "" {
		field(fieldPath, "o", func() { e.String(m.Path) })
	}
	if m.Interface != "" {
		field(fieldInterface, "s", func() { e.String(m.Interface) })
	}
	if m.Member != "" {
		field(fieldMember, "s", func() { e.String(m.Member) })
	}
	if m.ErrorName != "" {
		field(fieldErrorName, "s", func() { e.String(m.ErrorName) })
	}
	if m.Type == TypeReturn || m.Type == TypeError {
		field(fieldReplySerial, "u", func() { e.Uint32(m.ReplySerial) })
	}
	if m.Destination != "" {
		field(fieldDestination, "s", func() { e.String(m.Destination) })
	}
	if m.Sender != "" {
		field(fieldSender, "s", func() { e.String(m.Sender) })
	}
	if m.Signature != "" {
		field(fieldSignature, "g", func() { e.Signature(m.Signature) })
	}
	if m.unixFDCount > 0 {
		field(fieldUnixFDs, "u", func() { e.Uint32(uint32(m.unixFDCount)) })
	}
	e.EndArray(start)
	e.Align(8)
	return append(e.Bytes(), m.Body...)
}

var errBigEndian = errors.New("dbus: big-endian messages are not supported")

// messageLength is the length of the whole message whose fixed header starts
// buf, or 0 when buf does not yet hold that header.
func messageLength(buf []byte) (int, error) {
	if len(buf) < 16 {
		return 0, nil
	}
	if buf[0] != 'l' {
		return 0, errBigEndian
	}
	if buf[3] != 1 {
		return 0, fmt.Errorf("dbus: protocol version %d", buf[3])
	}
	body := int(binary.LittleEndian.Uint32(buf[4:]))
	fields := int(binary.LittleEndian.Uint32(buf[12:]))
	total := 16 + (fields+7)&^7 + body
	if fields > maxMessage || body > maxMessage || total > maxMessage {
		return 0, errors.New("dbus: message is too large")
	}
	return total, nil
}

// decodeMessage parses one whole message.
func decodeMessage(buf []byte) (*Message, error) {
	if len(buf) < 16 {
		return nil, ErrShort
	}
	m := &Message{Type: buf[1], Flags: buf[2]}
	bodyLength := int(binary.LittleEndian.Uint32(buf[4:]))
	m.Serial = binary.LittleEndian.Uint32(buf[8:])
	d := NewDecoder(buf)
	d.pos = 12
	end := d.ArrayLength(8)
	for d.Err() == nil && d.Pos() < end {
		d.Align(8)
		code := d.Byte()
		sig := d.Signature()
		switch {
		case code == fieldPath && sig == "o":
			m.Path = d.String()
		case code == fieldInterface && sig == "s":
			m.Interface = d.String()
		case code == fieldMember && sig == "s":
			m.Member = d.String()
		case code == fieldErrorName && sig == "s":
			m.ErrorName = d.String()
		case code == fieldReplySerial && sig == "u":
			m.ReplySerial = d.Uint32()
		case code == fieldDestination && sig == "s":
			m.Destination = d.String()
		case code == fieldSender && sig == "s":
			m.Sender = d.String()
		case code == fieldSignature && sig == "g":
			m.Signature = d.Signature()
		case code == fieldUnixFDs && sig == "u":
			m.unixFDCount = int(d.Uint32())
		default:
			d.Skip(sig) // a field this client has no use for
		}
	}
	if err := d.Err(); err != nil {
		return nil, err
	}
	d.pos = end
	d.Align(8)
	if d.pos+bodyLength != len(buf) {
		return nil, ErrShort
	}
	m.Body = buf[d.pos:]
	return m, nil
}
