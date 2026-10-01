package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	pb "github.com/NurPech/hannah-proto-go/v5/hannahv2"
)

// The device menu works on hannah.v2's typed devices: a device class plus slots. What a
// device offers is what its slots say — no BRIGHTNESS slot, no dimming; a read-only slot
// gets no action. Mirrors the Telegram bot's device_menu.py in the main Hannah repo.

var classIcons = map[pb.DeviceClass]string{
	pb.DeviceClass_DEVICE_CLASS_LIGHT:                 "💡",
	pb.DeviceClass_DEVICE_CLASS_SOCKET:                "🔌",
	pb.DeviceClass_DEVICE_CLASS_GENERIC_BINARY_SWITCH: "🔘",
	pb.DeviceClass_DEVICE_CLASS_THERMOSTAT:            "🌡️",
	pb.DeviceClass_DEVICE_CLASS_COVER:                 "↕️",
	pb.DeviceClass_DEVICE_CLASS_SENSOR:                "📊",
	pb.DeviceClass_DEVICE_CLASS_CLIMATE:               "❄️",
}

var classLabels = map[pb.DeviceClass]string{
	pb.DeviceClass_DEVICE_CLASS_LIGHT:                 "light",
	pb.DeviceClass_DEVICE_CLASS_SOCKET:                "socket",
	pb.DeviceClass_DEVICE_CLASS_GENERIC_BINARY_SWITCH: "switch",
	pb.DeviceClass_DEVICE_CLASS_THERMOSTAT:            "thermostat",
	pb.DeviceClass_DEVICE_CLASS_COVER:                 "cover",
	pb.DeviceClass_DEVICE_CLASS_SENSOR:                "sensor",
	pb.DeviceClass_DEVICE_CLASS_CLIMATE:               "climate",
}

// deviceIcon is the icon of the device class, a contact's by its subtype.
func deviceIcon(dev *pb.DeviceInfo) string {
	if dev.DeviceClass == pb.DeviceClass_DEVICE_CLASS_CONTACT {
		if dev.Subtype == pb.DeviceSubtype_DEVICE_SUBTYPE_DOOR {
			return "🚪"
		}
		return "🪟"
	}
	if icon, ok := classIcons[dev.DeviceClass]; ok {
		return icon
	}
	return "⚙️"
}

// deviceLabel is the device class as a word, a contact's by its subtype.
func deviceLabel(dev *pb.DeviceInfo) string {
	if dev.DeviceClass == pb.DeviceClass_DEVICE_CLASS_CONTACT {
		switch dev.Subtype {
		case pb.DeviceSubtype_DEVICE_SUBTYPE_DOOR:
			return "door"
		case pb.DeviceSubtype_DEVICE_SUBTYPE_WINDOW:
			return "window"
		default:
			return "contact"
		}
	}
	if label, ok := classLabels[dev.DeviceClass]; ok {
		return label
	}
	return "device"
}

func slotByKind(dev *pb.DeviceInfo, kind pb.SlotKind) *pb.Slot {
	for _, s := range dev.Slots {
		if s.Kind == kind {
			return s
		}
	}
	return nil
}

// statusDot is taken from the ON slot: on, off, or unknown (also without an ON slot).
func statusDot(dev *pb.DeviceInfo) string {
	on := slotByKind(dev, pb.SlotKind_SLOT_KIND_ON)
	if on == nil || on.Value == nil {
		return "⚫"
	}
	if b, ok := on.Value.Value.(*pb.SlotValue_Boolean); ok {
		if b.Boolean {
			return "🟢"
		}
		return "🔴"
	}
	return "⚫"
}

func slotName(s *pb.Slot) string {
	if s.Label != "" {
		return s.Label
	}
	return s.SlotId
}

// formatSlotValue renders the slot's current value; ok=false while it is unknown.
func formatSlotValue(s *pb.Slot) (string, bool) {
	if s.Value == nil {
		return "", false
	}
	var text string
	switch v := s.Value.Value.(type) {
	case *pb.SlotValue_Boolean:
		switch s.Kind {
		case pb.SlotKind_SLOT_KIND_ON:
			text = map[bool]string{true: "on", false: "off"}[v.Boolean]
		case pb.SlotKind_SLOT_KIND_OPEN:
			text = map[bool]string{true: "open", false: "closed"}[v.Boolean]
		default:
			text = strconv.FormatBool(v.Boolean)
		}
	case *pb.SlotValue_Number:
		text = strconv.FormatFloat(v.Number, 'f', -1, 64)
	case *pb.SlotValue_Text:
		text = v.Text
	case *pb.SlotValue_Rgb:
		return fmt.Sprintf("#%06X", v.Rgb&0xFFFFFF), true
	default:
		return "", false
	}
	if s.Unit != "" {
		text += " " + s.Unit
	}
	return text, true
}

// sortedSlots returns the device's slots by ID, for a stable, deterministic order.
func sortedSlots(dev *pb.DeviceInfo) []*pb.Slot {
	slots := append([]*pb.Slot(nil), dev.Slots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i].SlotId < slots[j].SlotId })
	return slots
}

// writableSlots are the slots a menu may offer (index i in the printed list always maps
// to writableSlots(dev)[i]).
func writableSlots(dev *pb.DeviceInfo) []*pb.Slot {
	var out []*pb.Slot
	for _, s := range sortedSlots(dev) {
		if s.Writable {
			out = append(out, s)
		}
	}
	return out
}

func isBoolKind(k pb.SlotKind) bool {
	switch k {
	case pb.SlotKind_SLOT_KIND_ON, pb.SlotKind_SLOT_KIND_OPEN, pb.SlotKind_SLOT_KIND_MOTION,
		pb.SlotKind_SLOT_KIND_STOP, pb.SlotKind_SLOT_KIND_GENERIC_BOOL:
		return true
	}
	return false
}

func isTextKind(k pb.SlotKind) bool {
	switch k {
	case pb.SlotKind_SLOT_KIND_MODE, pb.SlotKind_SLOT_KIND_FAN_SPEED, pb.SlotKind_SLOT_KIND_GENERIC_TEXT:
		return true
	}
	return false
}

func slotBool(s *pb.Slot) (value, known bool) {
	if s.Value == nil {
		return false, false
	}
	b, ok := s.Value.Value.(*pb.SlotValue_Boolean)
	if !ok {
		return false, false
	}
	return b.Boolean, true
}

func actionLabel(s *pb.Slot) string {
	if s.Kind == pb.SlotKind_SLOT_KIND_ON {
		if on, _ := slotBool(s); on {
			return "Turn off"
		}
		return "Turn on"
	}
	if cur, ok := formatSlotValue(s); ok {
		return fmt.Sprintf("Set %s (current: %s)", slotName(s), cur)
	}
	return fmt.Sprintf("Set %s", slotName(s))
}

// control says how a writable slot is operated.
type control int

const (
	controlToggle control = iota // boolean slot: flip it
	controlChoice                // pick one of a known list of values
	controlInput                 // type a value
)

func controlFor(s *pb.Slot) control {
	switch {
	case isBoolKind(s.Kind):
		return controlToggle
	case s.Kind == pb.SlotKind_SLOT_KIND_COLOR:
		return controlChoice
	case (s.Kind == pb.SlotKind_SLOT_KIND_MODE || s.Kind == pb.SlotKind_SLOT_KIND_FAN_SPEED) && len(s.Options) > 0:
		return controlChoice
	}
	return controlInput
}

// choice is one entry of a value list: what is shown, and the text slotValueFromText takes.
type choice struct {
	label string
	text  string
}

var colorChoices = []choice{
	{"red", "#FF0000"}, {"green", "#00FF00"}, {"blue", "#0000FF"}, {"yellow", "#FFFF00"}, {"white", "#FFFFFF"},
}

func choicesFor(s *pb.Slot) []choice {
	if s.Kind == pb.SlotKind_SLOT_KIND_COLOR {
		return colorChoices
	}
	out := make([]choice, 0, len(s.Options))
	for _, o := range s.Options {
		out = append(out, choice{label: o, text: o})
	}
	return out
}

// toggleValue is the value that flips a boolean slot (a STOP trigger is always true).
func toggleValue(s *pb.Slot) *pb.SlotValue {
	if s.Kind == pb.SlotKind_SLOT_KIND_STOP {
		return &pb.SlotValue{Value: &pb.SlotValue_Boolean{Boolean: true}}
	}
	on, _ := slotBool(s)
	return &pb.SlotValue{Value: &pb.SlotValue_Boolean{Boolean: !on}}
}

// slotValueFromText is the typed value for a slot from typed-in or listed text.
func slotValueFromText(s *pb.Slot, text string) (*pb.SlotValue, error) {
	text = strings.TrimSpace(text)
	switch {
	case isBoolKind(s.Kind):
		switch strings.ToLower(text) {
		case "true", "on", "yes":
			return &pb.SlotValue{Value: &pb.SlotValue_Boolean{Boolean: true}}, nil
		case "false", "off", "no":
			return &pb.SlotValue{Value: &pb.SlotValue_Boolean{Boolean: false}}, nil
		}
		return nil, fmt.Errorf("not a boolean: %q", text)
	case isTextKind(s.Kind):
		return &pb.SlotValue{Value: &pb.SlotValue_Text{Text: text}}, nil
	case s.Kind == pb.SlotKind_SLOT_KIND_COLOR:
		if len(text) != 7 || text[0] != '#' {
			return nil, fmt.Errorf("not a color: %q", text)
		}
		rgb, err := strconv.ParseUint(text[1:], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("not a color: %q", text)
		}
		return &pb.SlotValue{Value: &pb.SlotValue_Rgb{Rgb: uint32(rgb)}}, nil
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, fmt.Errorf("not a number: %q", text)
	}
	return &pb.SlotValue{Value: &pb.SlotValue_Number{Number: n}}, nil
}
