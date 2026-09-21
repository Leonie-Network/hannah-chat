package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/NurPech/hannah-proto-go/v4"
)

// deviceMenuTrustLevel mirrors the Telegram bot's _MENU_TRUST_MIN
// (telegram/hannah_telegram/bot.py) so both chat clients gate device
// control at the same trust level.
const deviceMenuTrustLevel = 7

// _CATEGORY_ICONS in bot.py, kept in sync — category names come from
// ioBroker (German) and are data, not translated project text.
var categoryIcons = map[string]string{
	"Licht":        "💡",
	"Stecker":      "🔌",
	"Temperaturen": "🌡️",
	"Fenster":      "🪟",
	"Helligkeit":   "☀️",
}

func categoryIcon(category string) string {
	if icon, ok := categoryIcons[category]; ok {
		return icon
	}
	return "⚙️"
}

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

	// valueKey/valueOptions are only valid during menuStepValueInput/menuStepEnumSelect:
	// the state key being set, and — for ENUM/COLOR — the ordered raw values behind
	// the numbered list just shown (index i -> valueOptions[i]).
	valueKey     string
	valueOptions []string
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
		fmt.Printf(" %d. %s %s %s\n", i+1, statusDot(dev), categoryIcon(dev.Category), dev.Name)
	}
	fmt.Println(" 0. Back")
	fmt.Println()
}

func statusDot(dev *pb.DeviceInfo) string {
	switch dev.Current["on"] {
	case "True":
		return "🟢"
	case "False":
		return "🔴"
	default:
		return "⚫"
	}
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

// writableActions returns the device's writable state keys, sorted for a
// stable, deterministic menu order (index i in the printed list always maps
// to writableActions(dev)[i]).
func writableActions(dev *pb.DeviceInfo) []string {
	keys := make([]string, 0, len(dev.States))
	for _, k := range dev.States {
		if dev.StateWritable[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func actionLabel(dev *pb.DeviceInfo, key string) string {
	if key == "on" && dev.StateTypes[key] == pb.StateType_BOOLEAN {
		if dev.Current["on"] == "True" {
			return "Turn off"
		}
		return "Turn on"
	}
	if cur, ok := dev.Current[key]; ok {
		return fmt.Sprintf("Set %s (current: %s)", key, cur)
	}
	return fmt.Sprintf("Set %s", key)
}

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

	fmt.Printf("%s (%s)\n", dev.Name, dev.Category)
	keys := make([]string, 0, len(dev.Current))
	for k := range dev.Current {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s: %s\n", k, dev.Current[k])
	}
	fmt.Println()

	actions := writableActions(dev)
	for i, key := range actions {
		fmt.Printf(" %d. %s\n", i+1, actionLabel(dev, key))
	}
	if len(actions) == 0 {
		fmt.Println("(no controllable states)")
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
	actions := writableActions(dev)
	if choice > len(actions) {
		fmt.Println("Invalid selection.")
		fmt.Println()
		showActions(s)
		return
	}
	key := actions[choice-1]

	switch dev.StateTypes[key] {
	case pb.StateType_BOOLEAN:
		value := "true"
		if dev.Current[key] == "True" {
			value = "false"
		}
		applyControl(s, dev.Id, key, value)
		showActions(s)

	case pb.StateType_ENUM, pb.StateType_COLOR:
		s.menu.valueKey = key
		s.menu.step = menuStepEnumSelect
		showEnumOptions(s, dev, key)

	default: // NUMERIC, TEXT, unspecified
		s.menu.valueKey = key
		s.menu.step = menuStepValueInput
		fmt.Printf("Enter new value for %s (empty to cancel): ", key)
	}
}

func applyControl(s *session, deviceID, key, value string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := s.client.ControlDevice(ctx, deviceID, key, value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "control failed: %v\n\n", err)
		return
	}
	if !resp.Ok {
		fmt.Printf("Control failed: %s\n\n", resp.Message)
	}
}

// ------------------------------------------------------------------
// Free-form value input (NUMERIC/TEXT)

func handleValueInput(s *session, line string) {
	value := strings.TrimSpace(line)
	key := s.menu.valueKey
	s.menu.valueKey = ""
	s.menu.step = menuStepActions

	if value == "" {
		fmt.Println("Cancelled.")
		fmt.Println()
		showActions(s)
		return
	}

	_, dev, ok := currentDeviceAndRoom(s)
	if !ok {
		return
	}
	applyControl(s, dev.Id, key, value)
	showActions(s)
}

// ------------------------------------------------------------------
// ENUM/COLOR value selection

func showEnumOptions(s *session, dev *pb.DeviceInfo, key string) {
	enum := dev.StateEnumValues[key]
	var raw []string
	if enum != nil {
		for v := range enum.Values {
			raw = append(raw, v)
		}
		sort.Strings(raw)
	}
	s.menu.valueOptions = raw

	fmt.Printf("Choose a value for %s:\n", key)
	for i, v := range raw {
		label := v
		if enum != nil {
			if l, ok := enum.Values[v]; ok && l != "" {
				label = l
			}
		}
		fmt.Printf(" %d. %s\n", i+1, label)
	}
	if len(raw) == 0 {
		fmt.Println("(no known values — back out and use another action)")
	}
	fmt.Println(" 0. Back")
	fmt.Println()
}

func handleEnumInput(s *session, line string) {
	choice, ok := parseChoice(line)
	if !ok {
		fmt.Println("Please enter a number from the list.")
		fmt.Println()
		return
	}
	if choice == 0 || choice > len(s.menu.valueOptions) {
		s.menu.valueKey = ""
		s.menu.valueOptions = nil
		s.menu.step = menuStepActions
		if choice != 0 {
			fmt.Println("Invalid selection.")
			fmt.Println()
		}
		showActions(s)
		return
	}

	value := s.menu.valueOptions[choice-1]
	key := s.menu.valueKey
	s.menu.valueKey = ""
	s.menu.valueOptions = nil
	s.menu.step = menuStepActions

	_, dev, ok := currentDeviceAndRoom(s)
	if !ok {
		return
	}
	applyControl(s, dev.Id, key, value)
	showActions(s)
}
