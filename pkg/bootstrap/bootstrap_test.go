package bootstrap

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-cli/pkg/plugins"
	"github.com/stripe/stripe-cli/pkg/plugins/proto"
)

// mockCoreCLIHelper is a test double for cliplugin.CoreCLIHelper.
type mockCoreCLIHelper struct {
	echoCalled bool
}

var _ plugins.CoreCLIHelper = &mockCoreCLIHelper{}

func (m *mockCoreCLIHelper) Echo(input string) (string, error) {
	m.echoCalled = true
	return input, nil
}

func (m *mockCoreCLIHelper) SendAnalytics(eventName string, eventValue string) error {
	return nil
}

func (m *mockCoreCLIHelper) KeychainGetPassword(key string) (string, bool, error) {
	return "", false, nil
}

func (m *mockCoreCLIHelper) KeychainSetPassword(key string, value string) error {
	return nil
}

func (m *mockCoreCLIHelper) KeychainDeletePassword(key string) (bool, error) {
	return false, nil
}

func (m *mockCoreCLIHelper) KeychainFindCredentials() ([]string, error) {
	return nil, nil
}

func (m *mockCoreCLIHelper) RunPeerPlugin(pluginName string, args []string, cwd string) error {
	return nil
}

func TestGetCoreCLIHelperNilByDefault(t *testing.T) {
	currentCoreCLIHelper = nil
	assert.Nil(t, GetCoreCLIHelper())
}

func TestPluginImplV3StoresCoreCLIHelper(t *testing.T) {
	currentCoreCLIHelper = nil
	rootCmdExecute = func(args []string) error { return nil }

	helper := &mockCoreCLIHelper{}
	impl := &PluginImplV3{}
	err := impl.RunCommand(&proto.AdditionalInfo{
		IsTerminal: &proto.IsTerminal{Stdin: true, Stdout: true, Stderr: true},
	}, []string{}, helper)

	require.NoError(t, err)
	assert.Same(t, helper, GetCoreCLIHelper())
}

func TestPluginImplV3PropagatesCommandError(t *testing.T) {
	currentCoreCLIHelper = nil
	expectedErr := errors.New("command failed")
	rootCmdExecute = func(args []string) error { return expectedErr }

	impl := &PluginImplV3{}
	err := impl.RunCommand(&proto.AdditionalInfo{
		IsTerminal: &proto.IsTerminal{Stdin: true, Stdout: true, Stderr: true},
	}, []string{"test"}, &mockCoreCLIHelper{})

	assert.ErrorIs(t, err, expectedErr)
}

func TestPluginImplV3PassesArgs(t *testing.T) {
	currentCoreCLIHelper = nil
	var receivedArgs []string
	rootCmdExecute = func(args []string) error {
		receivedArgs = args
		return nil
	}

	impl := &PluginImplV3{}
	err := impl.RunCommand(&proto.AdditionalInfo{
		IsTerminal: &proto.IsTerminal{Stdin: true, Stdout: true, Stderr: true},
	}, []string{"create", "--name", "test"}, &mockCoreCLIHelper{})

	require.NoError(t, err)
	assert.Equal(t, []string{"create", "--name", "test"}, receivedArgs)
}
