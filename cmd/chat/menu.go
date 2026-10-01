package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/NurPech/hannah-proto-go/v5/hannahv2"
)

// deviceMenuTrustLevel mirrors the Telegram bot's _MENU_TRUST_MIN
// (telegram/hannah_telegram/bot.py) so both chat clients gate device
// control at the same trust level.
const deviceMenuTrustLevel = 7

// menuStep is a single screen of the /devices menu.
type menuStep int

const (
	menuStepRooms menuStep = iota
	menuStepDevices
	menuStepActions
	menuStepValueInput
	menuStepEnumSelect
)

// deviceMenu is the state of an open /devices menu (see session.menu).
// Room/device identity is tracked as an index into a freshly-fetched
// GetDevices response — refetched on every screen, same trade-off the
// Telegram bot makes (room_idx/dev_idx in bot.py's callback_data).
type deviceMenu struct {
	step   menuStep
	room   int
	device int

	// valueSlot/valueChoices are only valid during menuStepValueInput/menuStepEnumSelect:
	// the ID of the slot being set, and — for a list of values — the choices behind the
	// numbered list just shown (index i -> valueChoices[i]).
	valueSlot    string
	valueChoices []choice
}

// startDeviceMenu opens the /devices menu at the room list.
func startDeviceMenu(s *session) {
	s.menu = &deviceMenu{step: menuStepRooms}
	showRooms(s)
}

// handleMenuInput consumes one input line while a menu is open.
func handleMenuInput(s *session, line string) {
	m := s.menu
	switch m.step {
	case menuStepRooms:
		handleRoomsInput(s, line)
	case menuStepDevices:
		handleDevicesInput(s, line)
	case menuStepActions:
		handleActionsInput(s, line)
	case menuStepValueInput:
		handleValueInput(s, line)
	case menuStepEnumSelect:
		handleEnumInput(s, line)
	}
}

func fetchDevices(s *session) (*pb.GetDevicesResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.client.GetDevices(ctx)
}

// parseChoice parses a menu selection, returning ok=false for anything that
// isn't a non-negative integer (including empty input).
func parseChoice(line string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func closeMenu(s *session) {
	s.menu = nil
	fmt.Println("Menu closed.")
	fmt.Println()
}

// ------------------------------------------------------------------
// Rooms

func showRooms(s *session) {
	resp, err := fetchDevices(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not load devices: %v\n\n", err)
		s.menu = nil
		return
	}
	if len(resp.Rooms) == 0 {
		fmt.Println("No rooms found.")
		fmt.Println()
		s.menu = nil
		return
	}
	fmt.Println("Devices — choose a room:")
	for i, room := range resp.Rooms {
		fmt.Printf(" %d. %s\n", i+1, room.Name)
	}
	fmt.Println(" 0. Close")
	fmt.Println()
}

func handleRoomsInput(s *session, line string) {
	choice, ok := parseChoice(line)
	if !ok {
		fmt.Println("Please enter a number from the list.")
		fmt.Println()
		showRooms(s)
		return
	}
	if choice == 0 {
		closeMenu(s)
		return
	}

	resp, err := fetchDevices(s)
	if err != nil || choice > len(resp.Rooms) {
		fmt.Println("Invalid selection.")
		fmt.Println()
		showRooms(s)
		return
	}

	s.menu.room = choice - 1
	s.menu.step = menuStepDevices
	showDevices(s)
}

// ------------------------------------------------------------------
// Devices

func showDevices(s *session) {
	resp, err := fetchDevices(s)
	if err != nil || s.menu.room >= len(resp.Rooms) {
		fmt.Println("Room no longer available.")
		fmt.Println()
		s.menu.step = menuStepRooms
		showRooms(s)
		return
	}
	room := resp.Rooms[s.menu.room]
	fmt.Printf("%s — devices:\n", room.Name)
	for i, dev := range room.Devices {
		fmt.Printf(" %d. %s %s %s\n", i+1, statusDot(dev), deviceIcon(dev), dev.Name)
	}
	fmt.Println(" 0. Back")
	fmt.Println()
}

func handleDevicesInput(s *session, line string) {
	choice, ok := parseChoice(line)
	if !ok {
		fmt.Println("Please enter a number from the list.")
		fmt.Println()
		showDevices(s)
		return
	}
	if choice == 0 {
		s.menu.step = menuStepRooms
		showRooms(s)
		return
	}

	resp, err := fetchDevices(s)
	if err != nil || s.menu.room >= len(resp.Rooms) {
		fmt.Println("Room no longer available.")
		fmt.Println()
		s.menu.step = menuStepRooms
		showRooms(s)
		return
	}
	room := resp.Rooms[s.menu.room]
	if choice > len(room.Devices) {
		fmt.Println("Invalid selection.")
		fmt.Println()
		showDevices(s)
		return
	}

	s.menu.device = choice - 1
	s.menu.step = menuStepActions
	showActions(s)
}

// ------------------------------------------------------------------
// Actions

// currentDeviceAndRoom re-fetches devices and resolves the menu's current
// room/device indices against it, falling back a screen if either no longer
// exists (device list changed underneath the open menu).
func currentDeviceAndRoom(s *session) (*pb.RoomInfo, *pb.DeviceInfo, bool) {
	resp, err := fetchDevices(s)
	if err != nil || s.menu.room >= len(resp.Rooms) {
		fmt.Println("Room no longer available.")
		fmt.Println()
		s.menu.step = menuStepRooms
		showRooms(s)
		return nil, nil, false
	}
	room := resp.Rooms[s.menu.room]
	if s.menu.device >= len(room.Devices) {
		fmt.Println("Device no longer available.")
		fmt.Println()
		s.menu.step = menuStepDevices
		showDevices(s)
		return nil, nil, false
	}
	return room, room.Devices[s.menu.device], true
}

func showActions(s *session) {
	_, dev, ok := currentDeviceAndRoom(s)
	if !ok {
		return
	}

	fmt.Printf("%s (%s)\n", dev.Name, deviceLabel(dev))
	for _, slot := range sortedSlots(dev) {
		if cur, ok := formatSlotValue(slot); ok {
			fmt.Printf("  %s: %s\n", slotName(slot), cur)
		}
	}
	fmt.Println()

	actions := writableSlots(dev)
	for i, slot := range actions {
		fmt.Printf(" %d. %s\n", i+1, actionLabel(slot))
	}
	if len(actions) == 0 {
		fmt.Println("(no controllable slots)")
	}
	fmt.Println(" 0. Back")
	fmt.Println()
}

func handleActionsInput(s *session, line string) {
	choice, ok := parseChoice(line)
	if !ok {
		fmt.Println("Please enter a number from the list.")
		fmt.Println()
		showActions(s)
		return
	}
	if choice == 0 {
		s.menu.step = menuStepDevices
		showDevices(s)
		return
	}

	_, dev, ok := currentDeviceAndRoom(s)
	if !ok {
		return
	}
	actions := writableSlots(dev)
	if choice > len(actions) {
		fmt.Println("Invalid selection.")
		fmt.Println()
		showActions(s)
		return
	}
	slot := actions[choice-1]

	switch controlFor(slot) {
	case controlToggle:
		applyControl(s, dev.Id, slot.SlotId, toggleValue(slot))
		showActions(s)

	case controlChoice:
		s.menu.valueSlot = slot.SlotId
		s.menu.step = menuStepEnumSelect
		showChoices(s, slot)

	default: // number or free text
		s.menu.valueSlot = slot.SlotId
		s.menu.step = menuStepValueInput
		fmt.Printf("Enter new value for %s (empty to cancel): ", slotName(slot))
	}
}

func applyControl(s *session, deviceID, slotID string, value *pb.SlotValue) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := s.client.ControlDevice(ctx, deviceID, slotID, value, s.sourceService, s.sourceUserID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "control failed: %v\n\n", err)
		return
	}
	if !resp.Ok {
		fmt.Printf("Control failed: %s\n\n", resp.Message)
	}
}

// findSlot is the device's slot with the given ID, or nil.
func findSlot(dev *pb.DeviceInfo, slotID string) *pb.Slot {
	for _, slot := range dev.Slots {
		if slot.SlotId == slotID {
			return slot
		}
	}
	return nil
}

// applyText sets the slot of the menu's current device to the typed or listed text.
func applyText(s *session, slotID, text string) {
	_, dev, ok := currentDeviceAndRoom(s)
	if !ok {
		return
	}
	slot := findSlot(dev, slotID)
	if slot == nil {
		fmt.Println("Slot no longer available.")
		fmt.Println()
		showActions(s)
		return
	}
	value, err := slotValueFromText(slot, text)
	if err != nil {
		fmt.Printf("Invalid value: %v\n\n", err)
		showActions(s)
		return
	}
	applyControl(s, dev.Id, slotID, value)
	showActions(s)
}

// ------------------------------------------------------------------
// Free-form value input (number/text)

func handleValueInput(s *session, line string) {
	text := strings.TrimSpace(line)
	slotID := s.menu.valueSlot
	s.menu.valueSlot = ""
	s.menu.step = menuStepActions

	if text == "" {
		fmt.Println("Cancelled.")
		fmt.Println()
		showActions(s)
		return
	}
	applyText(s, slotID, text)
}

// ------------------------------------------------------------------
// Value selection from a list (colors, a slot's options)

func showChoices(s *session, slot *pb.Slot) {
	choices := choicesFor(slot)
	s.menu.valueChoices = choices

	fmt.Printf("Choose a value for %s:\n", slotName(slot))
	for i, c := range choices {
		fmt.Printf(" %d. %s\n", i+1, c.label)
	}
	if len(choices) == 0 {
		fmt.Println("(no known values — back out and use another action)")
	}
	fmt.Println(" 0. Back")
	fmt.Println()
}

func handleEnumInput(s *session, line string) {
	n, ok := parseChoice(line)
	if !ok {
		fmt.Println("Please enter a number from the list.")
		fmt.Println()
		return
	}
	if n == 0 || n > len(s.menu.valueChoices) {
		s.menu.valueSlot = ""
		s.menu.valueChoices = nil
		s.menu.step = menuStepActions
		if n != 0 {
			fmt.Println("Invalid selection.")
			fmt.Println()
		}
		showActions(s)
		return
	}

	text := s.menu.valueChoices[n-1].text
	slotID := s.menu.valueSlot
	s.menu.valueSlot = ""
	s.menu.valueChoices = nil
	s.menu.step = menuStepActions
	applyText(s, slotID, text)
}
