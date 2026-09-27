package hannah

import (
	"context"
	"net"
	"testing"
	"time"

	legacy "github.com/NurPech/hannah-proto-go/v4"
	pb "github.com/NurPech/hannah-proto-go/v4/hannahv1"
	"google.golang.org/grpc"
)

// Fake Cores for one package each; SubmitText answers with the package's tag.

type v1Core struct {
	pb.UnimplementedHannahServiceServer
}

func (v1Core) GetSatellites(context.Context, *pb.Empty) (*pb.GetSatellitesResponse, error) {
	return &pb.GetSatellitesResponse{}, nil
}

func (v1Core) SubmitText(_ context.Context, req *pb.SubmitTextRequest) (*pb.SubmitTextResponse, error) {
	return &pb.SubmitTextResponse{Answer: "v1:" + req.Text}, nil
}

// ControlDevice echoes the requesting user so the test can check it arrives.
func (v1Core) ControlDevice(_ context.Context, req *pb.ControlDeviceRequest) (*pb.StatusResponse, error) {
	return &pb.StatusResponse{Ok: true, Message: req.SourceService + ":" + req.SourceUserId}, nil
}

type legacyCore struct {
	legacy.UnimplementedHannahServiceServer
}

func (legacyCore) GetSatellites(context.Context, *legacy.Empty) (*legacy.GetSatellitesResponse, error) {
	return &legacy.GetSatellitesResponse{}, nil
}

func (legacyCore) SubmitText(_ context.Context, req *legacy.SubmitTextRequest) (*legacy.SubmitTextResponse, error) {
	return &legacy.SubmitTextResponse{Answer: "legacy:" + req.Text}, nil
}

func startCore(t *testing.T, withV1, withLegacy bool) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	if withV1 {
		pb.RegisterHannahServiceServer(srv, v1Core{})
	}
	if withLegacy {
		legacy.RegisterHannahServiceServer(srv, legacyCore{})
	}
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func TestSubmitText_UsesThePathCoreServes(t *testing.T) {
	cases := []struct {
		name               string
		withV1, withLegacy bool
		want               string
	}{
		{"v1 only", true, false, "v1:hallo"},
		{"Core too old for v1", false, true, "legacy:hallo"},
		{"both, v1 wins", true, true, "v1:hallo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewClient(startCore(t, tc.withV1, tc.withLegacy))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()

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

// gessinger/voice/hannah#366: the requesting user travels with ControlDevice so
// Core can check the state's minimum trust level.
func TestControlDevice_SendsRequestingUser(t *testing.T) {
	c, err := NewClient(startCore(t, true, false))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.ControlDevice(ctx, "dev", "on", "true", "chat", "42")
	if err != nil {
		t.Fatalf("ControlDevice: %v", err)
	}
	if resp.Message != "chat:42" {
		t.Fatalf("source = %q, want %q", resp.Message, "chat:42")
	}
}
