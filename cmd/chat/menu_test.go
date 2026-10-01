package main

import (
	"testing"

	pb "github.com/NurPech/hannah-proto-go/v5/hannahv2"
)

func TestParseChoice(t *testing.T) {
	cases := []struct {
		in     string
		wantN  int
		wantOK bool
	}{
		{"0", 0, true},
		{"3", 3, true},
		{"  2  ", 2, true},
		{"", 0, false},
		{"-1", 0, false},
		{"abc", 0, false},
		{"1.5", 0, false},
	}
	for _, c := range cases {
		n, ok := parseChoice(c.in)
		if ok != c.wantOK || (ok && n != c.wantN) {
			t.Errorf("parseChoice(%q) = (%d, %v), want (%d, %v)", c.in, n, ok, c.wantN, c.wantOK)
		}
	}
}

func boolSlot(id string, kind pb.SlotKind, writable bool, value *bool) *pb.Slot {
	s := &pb.Slot{SlotId: id, Kind: kind, Writable: writable}
	if value != nil {
		s.Value = &pb.SlotValue{Value: &pb.SlotValue_Boolean{Boolean: *value}}
	}
	return s
}

func numSlot(id string, kind pb.SlotKind, writable bool, value float64) *pb.Slot {
	return &pb.Slot{SlotId: id, Kind: kind, Writable: writable, Value: &pb.SlotValue{Value: &pb.SlotValue_Number{Number: value}}}
}

func ptr[T any](v T) *T { return &v }

func slotIDs(slots []*pb.Slot) []string {
	var ids []string
	for _, s := range slots {
		ids = append(ids, s.SlotId)
	}
	return ids
}

func TestWritableSlotsSortedAndFiltered(t *testing.T) {
	dev := &pb.DeviceInfo{Slots: []*pb.Slot{
		boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, ptr(true)),
		numSlot("brightness", pb.SlotKind_SLOT_KIND_BRIGHTNESS, true, 40),
		{SlotId: "color", Kind: pb.SlotKind_SLOT_KIND_COLOR, Writable: false},
	}}
	got := slotIDs(writableSlots(dev))
	want := []string{"brightness", "on"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("writableSlots = %v, want %v", got, want)
	}
}

func TestActionLabelOnToggle(t *testing.T) {
	if got := actionLabel(boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, ptr(true))); got != "Turn off" {
		t.Errorf("actionLabel(on=true) = %q, want %q", got, "Turn off")
	}
	if got := actionLabel(boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, ptr(false))); got != "Turn on" {
		t.Errorf("actionLabel(on=false) = %q, want %q", got, "Turn on")
	}
}

func TestActionLabelOtherSlots(t *testing.T) {
	if got, want := actionLabel(numSlot("brightness", pb.SlotKind_SLOT_KIND_BRIGHTNESS, true, 75)), "Set brightness (current: 75)"; got != want {
		t.Errorf("actionLabel(brightness) = %q, want %q", got, want)
	}
	color := &pb.Slot{SlotId: "color", Kind: pb.SlotKind_SLOT_KIND_COLOR, Writable: true}
	if got := actionLabel(color); got != "Set color" {
		t.Errorf("actionLabel(color, unknown) = %q, want %q", got, "Set color")
	}
	generic := &pb.Slot{SlotId: "x1", Label: "Boost", Kind: pb.SlotKind_SLOT_KIND_GENERIC_NUMBER, Writable: true}
	if got := actionLabel(generic); got != "Set Boost" {
		t.Errorf("actionLabel(generic) = %q, want %q", got, "Set Boost")
	}
}

func TestStatusDot(t *testing.T) {
	cases := []struct {
		name string
		dev  *pb.DeviceInfo
		want string
	}{
		{"on", &pb.DeviceInfo{Slots: []*pb.Slot{boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, ptr(true))}}, "🟢"},
		{"off", &pb.DeviceInfo{Slots: []*pb.Slot{boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, ptr(false))}}, "🔴"},
		{"value unknown", &pb.DeviceInfo{Slots: []*pb.Slot{boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, nil)}}, "⚫"},
		{"no ON slot", &pb.DeviceInfo{Slots: []*pb.Slot{numSlot("temperature", pb.SlotKind_SLOT_KIND_TEMPERATURE, false, 21)}}, "⚫"},
	}
	for _, c := range cases {
		if got := statusDot(c.dev); got != c.want {
			t.Errorf("statusDot(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDeviceIconAndLabel(t *testing.T) {
	light := &pb.DeviceInfo{DeviceClass: pb.DeviceClass_DEVICE_CLASS_LIGHT}
	if deviceIcon(light) != "💡" || deviceLabel(light) != "light" {
		t.Errorf("light = %q/%q, want 💡/light", deviceIcon(light), deviceLabel(light))
	}
	door := &pb.DeviceInfo{DeviceClass: pb.DeviceClass_DEVICE_CLASS_CONTACT, Subtype: pb.DeviceSubtype_DEVICE_SUBTYPE_DOOR}
	if deviceIcon(door) != "🚪" || deviceLabel(door) != "door" {
		t.Errorf("door = %q/%q, want 🚪/door", deviceIcon(door), deviceLabel(door))
	}
	window := &pb.DeviceInfo{DeviceClass: pb.DeviceClass_DEVICE_CLASS_CONTACT, Subtype: pb.DeviceSubtype_DEVICE_SUBTYPE_WINDOW}
	if deviceIcon(window) != "🪟" || deviceLabel(window) != "window" {
		t.Errorf("window = %q/%q, want 🪟/window", deviceIcon(window), deviceLabel(window))
	}
	generic := &pb.DeviceInfo{DeviceClass: pb.DeviceClass_DEVICE_CLASS_GENERIC}
	if deviceIcon(generic) != "⚙️" || deviceLabel(generic) != "device" {
		t.Errorf("generic = %q/%q, want ⚙️/device", deviceIcon(generic), deviceLabel(generic))
	}
}

func TestControlFor(t *testing.T) {
	cases := []struct {
		name string
		slot *pb.Slot
		want control
	}{
		{"on", &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_ON}, controlToggle},
		{"color", &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_COLOR}, controlChoice},
		{"mode with options", &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_MODE, Options: []string{"cool", "dry"}}, controlChoice},
		{"mode without options", &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_MODE}, controlInput},
		{"brightness", &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_BRIGHTNESS}, controlInput},
	}
	for _, c := range cases {
		if got := controlFor(c.slot); got != c.want {
			t.Errorf("controlFor(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestToggleValue(t *testing.T) {
	if !toggleValue(boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, ptr(false))).GetBoolean() {
		t.Error("toggling an off slot should turn it on")
	}
	if toggleValue(boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, ptr(true))).GetBoolean() {
		t.Error("toggling an on slot should turn it off")
	}
	if !toggleValue(boolSlot("stop", pb.SlotKind_SLOT_KIND_STOP, true, nil)).GetBoolean() {
		t.Error("a STOP trigger is always true")
	}
}

func TestSlotValueFromText(t *testing.T) {
	brightness := &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_BRIGHTNESS}
	if v, err := slotValueFromText(brightness, " 30 "); err != nil || v.GetNumber() != 30 {
		t.Errorf("brightness 30 = %v, %v", v, err)
	}
	if _, err := slotValueFromText(brightness, "hell"); err == nil {
		t.Error("brightness \"hell\" should be rejected")
	}

	color := &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_COLOR}
	if v, err := slotValueFromText(color, "#FF8000"); err != nil || v.GetRgb() != 0xFF8000 {
		t.Errorf("color #FF8000 = %v, %v", v, err)
	}
	if _, err := slotValueFromText(color, "FF8000"); err == nil {
		t.Error("color without # should be rejected")
	}

	mode := &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_MODE}
	if v, err := slotValueFromText(mode, "cool"); err != nil || v.GetText() != "cool" {
		t.Errorf("mode cool = %v, %v", v, err)
	}

	on := &pb.Slot{Kind: pb.SlotKind_SLOT_KIND_ON}
	if v, err := slotValueFromText(on, "true"); err != nil || !v.GetBoolean() {
		t.Errorf("on true = %v, %v", v, err)
	}
	if _, err := slotValueFromText(on, "vielleicht"); err == nil {
		t.Error("on \"vielleicht\" should be rejected")
	}
}

func TestFormatSlotValue(t *testing.T) {
	cases := []struct {
		slot *pb.Slot
		want string
		ok   bool
	}{
		{boolSlot("on", pb.SlotKind_SLOT_KIND_ON, true, ptr(true)), "on", true},
		{boolSlot("open", pb.SlotKind_SLOT_KIND_OPEN, false, ptr(false)), "closed", true},
		{numSlot("temperature", pb.SlotKind_SLOT_KIND_TEMPERATURE, false, 21.5), "21.5", true},
		{&pb.Slot{Kind: pb.SlotKind_SLOT_KIND_COLOR, Value: &pb.SlotValue{Value: &pb.SlotValue_Rgb{Rgb: 0x0096FF}}}, "#0096FF", true},
		{&pb.Slot{Kind: pb.SlotKind_SLOT_KIND_ON}, "", false},
	}
	for _, c := range cases {
		got, ok := formatSlotValue(c.slot)
		if got != c.want || ok != c.ok {
			t.Errorf("formatSlotValue(%v) = (%q, %v), want (%q, %v)", c.slot.Kind, got, ok, c.want, c.ok)
		}
	}
}
