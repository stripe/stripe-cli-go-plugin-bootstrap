package bootstrap

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stripe/stripe-cli-go-plugin-bootstrap/v3/pkg/config"
	"github.com/stripe/stripe-cli-go-plugin-bootstrap/v3/pkg/telemetry"
	"github.com/stripe/stripe-cli/pkg/ansi"
	cliconfig "github.com/stripe/stripe-cli/pkg/config"
	cliplugin "github.com/stripe/stripe-cli/pkg/plugins"
	"github.com/stripe/stripe-cli/pkg/plugins/proto"
)

var stripeCliConfig cliconfig.Config

type PluginImpl struct {
	logger hclog.Logger
}

type Config struct {
	HandshakeKey   string
	HandshakeValue string
}

var rootCmdExecute func([]string) error

var hasFlags = false

func (g *PluginImpl) RunCommand(args []string) (string, error) {
	// cobra entry for plugin
	ctx, cancel := context.WithCancel(context.Background())
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGINT)
	errCh := make(chan error, 1)

	go func() {
		<-interruptCh
		cancel()
	}()

	go func() {
		errCh <- rootCmdExecute(args)
	}()

	select {
	case <-ctx.Done():
		return "", nil
	case err := <-errCh:
		return "", err
	}
}

type PluginImplGRPC struct {
}

func (p *PluginImplGRPC) RunCommand(additionalInfo *proto.AdditionalInfo, args []string) error {
	// cobra entry for plugin
	ctx, cancel := context.WithCancel(context.Background())
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGINT)
	errCh := make(chan error, 1)

	ansi.HostStdoutIsTerminal = additionalInfo.GetIsTerminal().GetStdout()
	ansi.HostStderrIsTerminal = additionalInfo.GetIsTerminal().GetStderr()

	go func() {
		<-interruptCh
		cancel()
	}()

	go func() {
		errCh <- rootCmdExecute(args)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

var currentCoreCLIHelper cliplugin.CoreCLIHelper

// GetCoreCLIHelper returns the CoreCLIHelper provided by the host CLI.
// Returns nil if the plugin was invoked via v1/v2 protocol or in dev mode.
func GetCoreCLIHelper() cliplugin.CoreCLIHelper {
	return currentCoreCLIHelper
}

type PluginImplV3 struct {
}

func (p *PluginImplV3) RunCommand(additionalInfo *proto.AdditionalInfo, args []string, coreCLIHelper cliplugin.CoreCLIHelper) error {
	currentCoreCLIHelper = coreCLIHelper

	ctx, cancel := context.WithCancel(context.Background())
	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGINT)
	errCh := make(chan error, 1)

	ansi.HostStdoutIsTerminal = additionalInfo.GetIsTerminal().GetStdout()
	ansi.HostStderrIsTerminal = additionalInfo.GetIsTerminal().GetStderr()

	go func() {
		<-interruptCh
		cancel()
	}()

	go func() {
		errCh <- rootCmdExecute(args)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// GetStripeCLIConfig returns a pointer to the CLI config that plugins can reference.
func GetStripeCLIConfig() *cliconfig.Config {
	return &stripeCliConfig
}

func registerGlobalFlags(pluginRootCmd *cobra.Command) {
	pluginRootCmd.PersistentFlags().StringVar(&stripeCliConfig.Profile.APIKey, "api-key", "", "Your API key to use for the command")
	pluginRootCmd.PersistentFlags().StringVar(&stripeCliConfig.Color, "color", "", "turn on/off color output (on, off, auto)")
	pluginRootCmd.PersistentFlags().StringVar(&stripeCliConfig.ProfilesFile, "config", "", "config file (default is $HOME/.config/stripe/config.toml)")
	pluginRootCmd.PersistentFlags().StringVar(&stripeCliConfig.Profile.DeviceName, "device-name", "", "device name")
	pluginRootCmd.PersistentFlags().StringVar(&stripeCliConfig.LogLevel, "log-level", "info", "log level (debug, info, trace, warn, error)")
	pluginRootCmd.PersistentFlags().StringVarP(&stripeCliConfig.Profile.ProfileName, "project-name", "p", "default", "the project name to read from for config")

	viper.BindPFlag("color", pluginRootCmd.PersistentFlags().Lookup("color"))
}

// StartPlugin defines the interfaces for the plugin and then serves them over RPC
// The CLI can then interact with the plugin from there
func StartPlugin(bc *Config, cmdExecute func(args []string, ctx context.Context) error, pluginRootCmd *cobra.Command, pluginInfo telemetry.PluginInfo) error {
	os.Setenv("CLIPLUGIN", "")

	// this is a workaround for running tests using this bootrapping function
	// flags cannot be registered against a command more than once
	if hasFlags == false {
		registerGlobalFlags(pluginRootCmd)
		hasFlags = true
	}

	stripeCliConfig.InitConfig()

	rootCmdExecute = telemetry.WithTelemetry(cmdExecute, pluginRootCmd, &stripeCliConfig, pluginInfo)

	if config.Devmode == true {
		var args []string

		if len(os.Args) > 1 {
			args = os.Args[1:]
		}

		return rootCmdExecute(args)
	} else {

		var handshakeConfig = hcplugin.HandshakeConfig{
			MagicCookieKey:   bc.HandshakeKey,
			MagicCookieValue: bc.HandshakeValue,
		}

		versionedPluginSetMap := map[int]hcplugin.PluginSet{
			1: {
				"main": &cliplugin.CLIPluginV1{Impl: &PluginImpl{}},
			},
			2: {
				"main": &cliplugin.CLIPluginGRPC{Impl: &PluginImplGRPC{}},
			},
			3: {
				"main": &cliplugin.CLIPluginV3{Impl: &PluginImplV3{}},
			},
		}

		hcplugin.Serve(&hcplugin.ServeConfig{
			HandshakeConfig:  handshakeConfig,
			VersionedPlugins: versionedPluginSetMap,
			GRPCServer:       hcplugin.DefaultGRPCServer,
		})
	}

	return nil
}
