package output

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stripe/stripe-cli/pkg/plugins"
	"github.com/stripe/stripe-cli/pkg/plugins/proto"
	"github.com/stripe/stripe-cli/pkg/plugins/sdk"
	"github.com/stripe/stripe-cli/pkg/stripe"

	"github.com/stripe/stripe-cli-go-plugin-bootstrap/v3/pkg/bootstrap"
)

// fakeHelper is a CoreCLIHelper that records what a command sent and can fail
// with a chosen error, standing in for the host CLI.
type fakeHelper struct {
	plugins.CoreCLIHelper

	outputs []*proto.SendCommandOutputRequest
	err     error
}

func (h *fakeHelper) SendCommandOutput(req *proto.SendCommandOutputRequest) error {
	h.outputs = append(h.outputs, req)
	return h.err
}

// withHelper installs a helper for the duration of a test, mirroring what the
// host does before invoking a command.
func withHelper(t *testing.T, helper plugins.CoreCLIHelper) {
	t.Helper()
	bootstrap.SetCoreCLIHelper(helper)
	t.Cleanup(func() { bootstrap.SetCoreCLIHelper(nil) })
}

// recordingTelemetry captures rollout events. SendEvent runs on its own
// goroutine, so events arrive over a channel.
type recordingTelemetry struct {
	events chan [2]string
}

func newRecordingTelemetry() *recordingTelemetry {
	return &recordingTelemetry{events: make(chan [2]string, 4)}
}

func (c *recordingTelemetry) SendEvent(_ context.Context, name, value string) {
	c.events <- [2]string{name, value}
}

func (c *recordingTelemetry) SendAPIRequestEvent(context.Context, string, bool) (*http.Response, error) {
	return nil, nil
}

func (c *recordingTelemetry) next(t *testing.T) [2]string {
	t.Helper()
	select {
	case event := <-c.events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("no telemetry event was sent")
		return [2]string{}
	}
}

func (c *recordingTelemetry) requireNone(t *testing.T) {
	t.Helper()
	select {
	case event := <-c.events:
		t.Fatalf("unexpected telemetry event %v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func telemetryContext(client stripe.TelemetryClient) context.Context {
	ctx := stripe.WithTelemetryClient(context.Background(), client)
	return stripe.WithEventMetadata(ctx, stripe.NewEventMetadata())
}

func TestCLISendsThroughHostHelper(t *testing.T) {
	helper := &fakeHelper{}
	withHelper(t, helper)

	require.True(t, Available())
	require.NoError(t, CLI().Output("apps upload", sdk.Data(map[string]string{"app_id": "app_123"})))

	require.Len(t, helper.outputs, 1)
	require.Equal(t, "apps upload", helper.outputs[0].GetCommand())
}

// Without a host helper the plugin is on its own: every call must report a
// fallback rather than panicking.
func TestNoHelperFallsBack(t *testing.T) {
	withHelper(t, nil)

	require.False(t, Available())

	err := CLI().Output("apps upload", sdk.Data(map[string]string{"app_id": "app_123"}))
	require.ErrorIs(t, err, sdk.ErrNoHelper)

	client := newRecordingTelemetry()
	require.True(t, Fallback(telemetryContext(client), "apps upload", err))
	require.Equal(t, [2]string{eventFallback, "apps upload: " + reasonNoHelper}, client.next(t))
}

// An older core CLI answers Unimplemented, having rendered nothing, so the
// plugin renders locally.
func TestUnimplementedFallsBack(t *testing.T) {
	client := newRecordingTelemetry()
	err := status.Error(codes.Unimplemented, "unknown method SendCommandOutput")

	require.True(t, Fallback(telemetryContext(client), "apps upload", err))
	require.Equal(t, [2]string{eventFallback, "apps upload: " + reasonUnimplemented}, client.next(t))
}

// Any other failure may have arrived after the core already rendered, so the
// command must surface the error instead of printing a duplicate.
func TestOtherFailuresDoNotFallBack(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"unavailable", status.Error(codes.Unavailable, "connection closed")},
		{"internal", status.Error(codes.Internal, "boom")},
		{"deadline exceeded", status.Error(codes.DeadlineExceeded, "too slow")},
		{"plain error", errors.New("boom")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newRecordingTelemetry()
			require.False(t, Fallback(telemetryContext(client), "apps upload", tc.err))

			event := client.next(t)
			require.Equal(t, eventFailed, event[0])
			require.Contains(t, event[1], "apps upload: ")
		})
	}
}

func TestSuccessIsNotAFallback(t *testing.T) {
	client := newRecordingTelemetry()
	require.False(t, Fallback(telemetryContext(client), "apps upload", nil))
	client.requireNone(t)
}

// Telemetry is best-effort: a context without a client must not break output.
func TestFallbackWorksWithoutTelemetry(t *testing.T) {
	require.True(t, Fallback(context.Background(), "apps upload", sdk.ErrNoHelper))
	require.False(t, Fallback(context.Background(), "apps upload", errors.New("boom")))
}
