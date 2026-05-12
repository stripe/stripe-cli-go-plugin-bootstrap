package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stripe/stripe-cli/pkg/stripe"
	"github.com/stripe/stripe-cli/pkg/useragent"

	cliconfig "github.com/stripe/stripe-cli/pkg/config"
)

type PluginInfo struct {
	Name    string
	Version string
}

// WithTelemetry wraps the plugin's main Execute function such that it will record basic telemetry events.
func WithTelemetry(cmdExecute func(args []string, ctx context.Context) error, cmd *cobra.Command, cfg *cliconfig.Config, pluginInfo PluginInfo) func(args []string) error {
	isTelemetryOptedOut := stripe.TelemetryOptedOut(os.Getenv("STRIPE_CLI_TELEMETRY_OPTOUT")) || stripe.TelemetryOptedOut(os.Getenv("DO_NOT_TRACK"))

	if isTelemetryOptedOut {
		return func(args []string) error {
			return cmdExecute(args, context.Background())
		}
	}

	setupTelemetryHook(cmd, cfg, pluginInfo)
	ctx := setupContextWithTelemetry(context.Background())

	return func(args []string) error {
		defer awaitTelemetryCompleted(ctx)
		return cmdExecute(args, ctx)
	}
}

func setupTelemetryHook(cmd *cobra.Command, cfg *cliconfig.Config, pluginInfo PluginInfo) {
	// Bugs in telemetry should not affect whether the plugin works or not.
	// We must fail open whenever possible.
	if cmd == nil {
		return
	}

	nextPersistentPreRunE := cmd.PersistentPreRunE
	nextPersistentPreRun := cmd.PersistentPreRun

	// We must use the error version of this hook because it takes precedence over the non-error
	// one in cobra, and we don't want plugin authors to be able to override our hook.
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		var merchant string
		if cfg != nil {
			merchant, _ = cfg.Profile.GetAccountID()
		}
		telemetryMetadata := stripe.GetEventMetadata(cmd.Context())
		telemetryMetadata.SetCommandPath(resolveCommandPath(cmd))
		telemetryMetadata.SetMerchant(merchant)
		telemetryMetadata.SetUserAgent(resolveUserAgent(pluginInfo))

		sendCommandInvocationEvent(cmd.Context())

		// Finally we want our hook to be transparent to plugin authors, so we explicitly call the
		// plugin author's hooks following the cobra behavior that the error version takes precendence.
		if nextPersistentPreRunE != nil {
			return nextPersistentPreRunE(cmd, args)
		} else if nextPersistentPreRun != nil {
			nextPersistentPreRun(cmd, args)
			return nil
		} else {
			return nil
		}
	}
}

func resolveCommandPath(cmd *cobra.Command) string {
	// The mainline CLI removes the argv[0] when it calls into the plugin, so add it back.
	return fmt.Sprintf("stripe %s", cmd.CommandPath())
}

func resolveUserAgent(pluginInfo PluginInfo) string {
	cliUserAgent := useragent.GetEncodedUserAgent()

	var pluginUserAgent string
	if pluginInfo.Name == "" {
		pluginUserAgent = "plugin_unknown"
	} else {
		pluginUserAgent = pluginInfo.Name
		if pluginInfo.Version != "" {
			pluginUserAgent += "/" + pluginInfo.Version
		}
	}

	return fmt.Sprintf("%s %s", cliUserAgent, pluginUserAgent)
}

func sendCommandInvocationEvent(ctx context.Context) {
	telemetryClient := stripe.GetTelemetryClient(ctx)
	if telemetryClient != nil {
		go telemetryClient.SendEvent(ctx, "Command Invoked", "Cobra")
	}
}

func awaitTelemetryCompleted(ctx context.Context) {
	telemetryClient := stripe.GetTelemetryClient(ctx)
	if telemetryClient == nil {
		return
	}
	if analyticsTelemetryClient, ok := telemetryClient.(*stripe.AnalyticsTelemetryClient); ok {
		analyticsTelemetryClient.Wait()
	}
}

func setupContextWithTelemetry(ctx context.Context) context.Context {
	httpClient := &http.Client{
		Timeout: time.Second * 3,
	}
	telemetryClient := &stripe.AnalyticsTelemetryClient{HTTPClient: httpClient}

	contextWithTelemetry := stripe.WithTelemetryClient(ctx, telemetryClient)

	telemetryMetadata := stripe.NewEventMetadata()
	updatedCtx := stripe.WithEventMetadata(contextWithTelemetry, telemetryMetadata)

	return updatedCtx
}
