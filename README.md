# stripe-cli-plugin-bootstrap

A Go library that bootstraps [Stripe CLI](https://github.com/stripe/stripe-cli) plugins. It handles the plugin lifecycle including RPC communication with the CLI host, command registration, telemetry, and signal handling.

## Usage

```go
package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/stripe/stripe-cli-go-plugin-bootstrap/v2/pkg/bootstrap"
	"github.com/stripe/stripe-cli-go-plugin-bootstrap/v2/pkg/telemetry"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "myplugin",
		Short: "My Stripe CLI plugin",
	}

	bootstrap.StartPlugin(
		&bootstrap.Config{
			HandshakeKey:   "myplugin",
			HandshakeValue: "hello",
		},
		func(args []string, ctx context.Context) error {
			rootCmd.SetArgs(args)
			return rootCmd.ExecuteContext(ctx)
		},
		rootCmd,
		telemetry.PluginInfo{
			Name:    "myplugin",
			Version: "0.1.0",
		},
	)
}
```

## Features

- **Plugin RPC**: Handles hashicorp/go-plugin communication (v1 net/rpc and v2 gRPC protocols)
- **Telemetry**: Opt-in CLI analytics via `WithTelemetry`
- **Signal handling**: Graceful cancellation on SIGINT
- **CLI config**: Automatically registers global flags (`--api-key`, `--color`, `--config`, etc.)
- **Dev mode**: Build with `-tags localdev` to run the plugin standalone without the CLI host
- **Command tree extraction**: `ExtractCommandTree` exports your command structure for use in plugin manifests

## Installation

```
go get github.com/stripe/stripe-cli-go-plugin-bootstrap/v2
```

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.
