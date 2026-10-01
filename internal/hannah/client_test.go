package hannah

import (
	"context"
	"net"
	"testing"
	"time"

	v1pb "github.com/NurPech/hannah-proto-go/v5/hannahv1"
	pb "github.com/NurPech/hannah-proto-go/v5/hannahv2"
	"google.golang.org/grpc"
)

// Fake Cores for one generation each; SubmitText answers with the generation's tag.

type v2Core struct {
	pb.UnimplementedHannahServiceServer
}

func (v2Core) GetSatellites(context.Context, *pb.Empty) (*pb.GetSatellitesResponse, error) {
	return &pb.GetSatellitesResponse{}, nil
}

func (v2Core) SubmitText(_ context.Context, req *pb.SubmitTextRequest) (*pb.SubmitTextResponse, error) {
	return &pb.SubmitTextResponse{Answer: "v2:" + req.Text}, nil
}

// ControlDevice echoes the slot, the value and the requesting user so the test can check
// they arrive.
func (v2Core) ControlDevice(_ context.Context, req *pb.ControlDeviceRequest) (*pb.StatusResponse, error) {
	return &pb.StatusResponse{Ok: true, Message: req.SlotId + "=" + req.GetValue().GetText() + "|" + req.SourceService + ":" + req.SourceUserId}, nil
}

func (v2Core) GetDevices(context.Context, *pb.Empty) (*pb.GetDevicesResponse, error) {
	return &pb.GetDevicesResponse{Rooms: []*pb.RoomInfo{{Key: "kueche", Name: "Küche"}}}, nil
}

// v1Core is a Core too old for hannah.v2: it only serves hannah.v1.
type v1Core struct {
	v1pb.UnimplementedHannahServiceServer
}

func (v1Core) GetSatellites(context.Context, *v1pb.Empty) (*v1pb.GetSatellitesResponse, error) {
	return &v1pb.GetSatellitesResponse{}, nil
}

func (v1Core) SubmitText(_ context.Context, req *v1pb.SubmitTextRequest) (*v1pb.SubmitTextResponse, error) {
	return &v1pb.SubmitTextResponse{Answer: "v1:" + req.Text}, nil
}

// ControlDevice echoes state, value and requesting user as hannah.v1 received them.
func (v1Core) ControlDevice(_ context.Context, req *v1pb.ControlDeviceRequest) (*v1pb.StatusResponse, error) {
	return &v1pb.StatusResponse{Ok: true, Message: req.State + "=" + req.Value + "|" + req.SourceService + ":" + req.SourceUserId}, nil
}

func (v1Core) GetDevices(context.Context, *v1pb.Empty) (*v1pb.GetDevicesResponse, error) {
	return &v1pb.GetDevicesResponse{Rooms: []*v1pb.RoomInfo{{
		Key: "kueche", Name: "Küche",
		Devices: []*v1pb.DeviceInfo{{
			Id: "lampe", Name: "Lampe", Category: "light",
			States:        []string{"on"},
			StateWritable: map[string]bool{"on": true},
			StateTypes:    map[string]v1pb.StateType{"on": v1pb.StateType_BOOLEAN},
			Current:       map[string]string{"on": "True"},
		}},
	}}}, nil
}

func startCore(t *testing.T, withV2, withV1 bool) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	if withV2 {
		pb.RegisterHannahServiceServer(srv, v2Core{})
	}
	if withV1 {
		v1pb.RegisterHannahServiceServer(srv, v1Core{})
	}
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func newTestClient(t *testing.T, withV2, withV1 bool) *Client {
	t.Helper()
	c, err := NewClient(startCore(t, withV2, withV1))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

func TestSubmitText_UsesThePathCoreServes(t *testing.T) {
	cases := []struct {
		name           string
		withV2, withV1 bool
		want           string
	}{
		{"v2 only", true, false, "v2:hallo"},
		{"Core too old for v2", false, true, "v1:hallo"},
		{"both, v2 wins", true, true, "v2:hallo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.withV2, tc.withV1)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			resp, err := c.SubmitText(ctx, "hallo", "chat", "1")
			if err != nil {
				t.Fatalf("SubmitText: %v", err)
			}
			if resp.Answer != tc.want {
				t.Fatalf("answer = %q, want %q", resp.Answer, tc.want)
			}
		})
	}
}

func textValue(s string) *pb.SlotValue {
	return &pb.SlotValue{Value: &pb.SlotValue_Text{Text: s}}
}

func boolValue(b bool) *pb.SlotValue {
	return &pb.SlotValue{Value: &pb.SlotValue_Boolean{Boolean: b}}
}

// gessinger/voice/hannah#366: the requesting user travels with ControlDevice so
// Core can check the slot's minimum trust level.
func TestControlDevice_SendsRequestingUser(t *testing.T) {
	c := newTestClient(t, true, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.ControlDevice(ctx, "dev", "mode", textValue("cool"), "chat", "42")
	if err != nil {
		t.Fatalf("ControlDevice: %v", err)
	}
	if want := "mode=cool|chat:42"; resp.Message != want {
		t.Fatalf("message = %q, want %q", resp.Message, want)
	}
}

// Against a Core without hannah.v2 the slot and its typed value are translated to the
// hannah.v1 state key and string, and the requesting user still arrives.
func TestControlDevice_IsTranslatedForACoreWithoutV2(t *testing.T) {
	c := newTestClient(t, false, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.ControlDevice(ctx, "lampe", "on", boolValue(true), "chat", "42")
	if err != nil {
		t.Fatalf("ControlDevice: %v", err)
	}
	if want := "on=true|chat:42"; resp.Message != want {
		t.Fatalf("message = %q, want %q", resp.Message, want)
	}
}

// Against a Core without hannah.v2 the v1 states come back as typed slots.
func TestGetDevices_IsTranslatedForACoreWithoutV2(t *testing.T) {
	c := newTestClient(t, false, true)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.GetDevices(ctx)
	if err != nil {
		t.Fatalf("GetDevices: %v", err)
	}
	if len(resp.Rooms) != 1 || len(resp.Rooms[0].Devices) != 1 {
		t.Fatalf("rooms = %v, want one room with one device", resp.Rooms)
	}
	dev := resp.Rooms[0].Devices[0]
	if dev.Id != "lampe" || dev.DeviceClass != pb.DeviceClass_DEVICE_CLASS_LIGHT {
		t.Fatalf("device = %v, want lampe as a light", dev)
	}
	on := findSlotByKind(dev, pb.SlotKind_SLOT_KIND_ON)
	if on == nil || !on.Writable || !on.GetValue().GetBoolean() {
		t.Fatalf("on slot = %v, want a writable slot with value true", on)
	}
}

func findSlotByKind(dev *pb.DeviceInfo, kind pb.SlotKind) *pb.Slot {
	for _, s := range dev.Slots {
		if s.Kind == kind {
			return s
		}
	}
	return nil
}
