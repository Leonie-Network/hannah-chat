// Package hannah provides a gRPC client to Hannah Core.
package hannah

import (
	"context"
	"fmt"

	pb "github.com/NurPech/hannah-proto-go/v4/hannahv1"
	"gitlab.com/gessinger/hannah-grpc-lib/go/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Client is a gRPC client to Hannah Core. It works with hannah.v1 types only; against a
// Core too old for hannah.v1, calls go to the unversioned N−1 path instead (the probe and
// the one-time warning come from hannah-grpc-lib's client package).
type Client struct {
	conn *grpc.ClientConn
	stub pb.HannahServiceClient
}

// NewClient dials Hannah Core at address (e.g. "192.0.2.1:50051").
func NewClient(address string) (*Client, error) {
	// client.DialOptions: insecure transport, x-proto-version and x-compat-version
	// (hannah-proto#9/hannah#217) on every call, x-compat-version per service path.
	opts := append(client.DialOptions(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32*1024*1024)),
	)
	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %q: %w", address, err)
	}
	return &Client{conn: conn, stub: pb.NewHannahServiceClient(client.New(conn, nil))}, nil
}

// Close tears down the gRPC connection.
func (c *Client) Close() {
	c.conn.Close()
}

// WaitReady blocks until the connection is ready, or returns ctx's error if it
// isn't by the time ctx is done — lets callers fail fast on an unreachable
// Hannah Core instead of discovering it on the first SubmitText call.
func (c *Client) WaitReady(ctx context.Context) error {
	c.conn.Connect()
	for {
		state := c.conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if !c.conn.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
}

// SubmitText sends a text command to Hannah Core, as if spoken by sourceUserID
// through sourceService, and returns the matched intent and Hannah's answer.
func (c *Client) SubmitText(ctx context.Context, text, sourceService, sourceUserID string) (*pb.SubmitTextResponse, error) {
	return c.stub.SubmitText(ctx, &pb.SubmitTextRequest{
		Text:          text,
		SourceService: sourceService,
		SourceUserId:  sourceUserID,
	})
}

// Login authenticates against Hannah's user registry. resp.Found is false on
// wrong credentials (not an error) — resp.User is only valid when Found is true.
func (c *Client) Login(ctx context.Context, username, password string) (*pb.UserResponse, error) {
	return c.stub.Login(ctx, &pb.LoginRequest{
		Username: username,
		Password: password,
	})
}

// GetDevices returns every room and its devices, including writable states
// and their types — the data the /devices menu is built from.
func (c *Client) GetDevices(ctx context.Context) (*pb.GetDevicesResponse, error) {
	return c.stub.GetDevices(ctx, &pb.Empty{})
}

// ControlDevice sets a single device state directly (bypassing NLU), e.g.
// state="on" value="true", or state="level" value="50". sourceService/sourceUserID
// identify the requesting user like on SubmitText, so Core can check the state's
// minimum trust level (gessinger/voice/hannah#366); unknown = guest.
func (c *Client) ControlDevice(ctx context.Context, deviceID, state, value, sourceService, sourceUserID string) (*pb.StatusResponse, error) {
	return c.stub.ControlDevice(ctx, &pb.ControlDeviceRequest{
		DeviceId:      deviceID,
		State:         state,
		Value:         value,
		SourceService: sourceService,
		SourceUserId:  sourceUserID,
	})
}

// LinkAccount ties a roomie to an external account under the given provider —
// used after Login to self-link the roomie's own ID under provider "chat"
// (gessinger/voice/hannah#332), so later SubmitText calls with the same
// source_service/source_user_id resolve back to that roomie.
func (c *Client) LinkAccount(ctx context.Context, userID int32, service, accountID string) (*pb.StatusResponse, error) {
	return c.stub.LinkAccount(ctx, &pb.LinkAccountRequest{
		UserId:    userID,
		Service:   service,
		AccountId: accountID,
	})
}
