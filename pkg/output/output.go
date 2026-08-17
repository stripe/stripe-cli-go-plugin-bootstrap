// Package output lets a plugin send its output to the core CLI for rendering,
// with one precise rule for when the plugin must render locally instead.
//
// A plugin's command sends output through CLI(), then decides what to do with
// the error:
//
//	cli := output.CLI()
//	if err := cli.Output("apps upload", sdk.Data(result)); err != nil {
//	    if !output.Fallback(ctx, "apps upload", err) {
//	        return err
//	    }
//	    printResultLocally(result)
//	}
//
// Fallback is true only when nothing was rendered: no helper (v1/v2 protocol or
// dev mode), or a core CLI too old to know the RPC. Any other error is the
// command's error to return, because the core may have rendered the output
// already and printing locally would duplicate it.
package output

import (
	"context"
	"errors"

	"github.com/stripe/stripe-cli/pkg/plugins/sdk"
	"github.com/stripe/stripe-cli/pkg/stripe"

	"github.com/stripe/stripe-cli-go-plugin-bootstrap/v3/pkg/bootstrap"
)

// Telemetry event names for the centralized-output rollout. Fallbacks are
// expected during rollout; failures are not, and are the signal to watch.
const (
	eventFallback = "Centralized Output Fallback"
	eventFailed   = "Centralized Output Failed"
)

// Reasons reported as the telemetry event value.
const (
	reasonNoHelper      = "no_helper"
	reasonUnimplemented = "unimplemented"
)

// CLI returns an SDK bound to the helper the host CLI provided.
//
// The helper is only available once the host has invoked the command, so call
// this from inside the command rather than at package init. When no helper is
// available the returned CLI is still usable: its methods report ErrNoHelper,
// which Fallback treats as "render locally".
func CLI() *sdk.CLI {
	return sdk.New(bootstrap.GetCoreCLIHelper())
}

// Available reports whether the host CLI can render output for this plugin.
//
// Use it to choose an output implementation up front. Individual calls can still
// fail against a core CLI that is too old for a given RPC, so check the error
// from each call with Fallback as well.
func Available() bool {
	return bootstrap.GetCoreCLIHelper() != nil
}

// Fallback reports whether err means the plugin should render this output
// locally, and records telemetry either way.
//
// It is true only for the two cases where the core rendered nothing: no helper
// at all, or a core CLI that predates the RPC. Every other error means the
// output may already be on screen, so the caller must surface the error instead
// of printing a second copy.
//
// A nil error is not a fallback.
func Fallback(ctx context.Context, command string, err error) bool {
	if err == nil {
		return false
	}

	if sdk.Unsupported(err) {
		reason := reasonUnimplemented
		if errors.Is(err, sdk.ErrNoHelper) {
			reason = reasonNoHelper
		}
		report(ctx, eventFallback, command, reason)
		return true
	}

	report(ctx, eventFailed, command, err.Error())
	return false
}

// report sends a rollout event. Telemetry must never change what the command
// does, so a missing client is simply ignored.
//
// The command travels in the event value rather than the event metadata, which
// the telemetry hook owns and populates from cobra.
func report(ctx context.Context, event, command, detail string) {
	client := stripe.GetTelemetryClient(ctx)
	if client == nil {
		return
	}

	value := detail
	if command != "" {
		value = command + ": " + detail
	}

	// Sent in the background so rendering is never delayed by a network call;
	// the plugin's telemetry wrapper waits for in-flight events before exit.
	go client.SendEvent(ctx, event, value)
}
