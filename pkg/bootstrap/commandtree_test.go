package bootstrap

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCmd(use, short string) *cobra.Command {
	return &cobra.Command{Use: use, Short: short}
}

func TestExtractCommandTree(t *testing.T) {
	root := newCmd("myapp", "My app plugin")
	create := newCmd("create", "Create something")
	logs := newCmd("logs", "View logs")
	tail := newCmd("tail", "Tail logs")
	logs.AddCommand(tail)
	root.AddCommand(create, logs)

	tree := ExtractCommandTree(root)

	require.Equal(t, 2, len(tree))
	assert.Equal(t, "create", tree[0].Name)
	assert.Equal(t, "Create something", tree[0].Desc)
	assert.Nil(t, tree[0].Commands)

	assert.Equal(t, "logs", tree[1].Name)
	require.Equal(t, 1, len(tree[1].Commands))
	assert.Equal(t, "tail", tree[1].Commands[0].Name)
	assert.Equal(t, "Tail logs", tree[1].Commands[0].Desc)
}

func TestExtractCommandTreeExcludesHidden(t *testing.T) {
	root := newCmd("myapp", "")
	visible := newCmd("visible", "Visible")
	hidden := newCmd("hidden", "Hidden")
	hidden.Hidden = true
	root.AddCommand(visible, hidden)

	tree := ExtractCommandTree(root)

	require.Equal(t, 1, len(tree))
	assert.Equal(t, "visible", tree[0].Name)
}

func TestExtractCommandTreeExcludesHelp(t *testing.T) {
	root := newCmd("myapp", "")
	child := newCmd("child", "A child")
	root.AddCommand(child)
	root.InitDefaultHelpCmd()

	tree := ExtractCommandTree(root)

	require.Equal(t, 1, len(tree))
	assert.Equal(t, "child", tree[0].Name)
}

func TestExtractCommandTreeEmpty(t *testing.T) {
	root := newCmd("myapp", "")

	tree := ExtractCommandTree(root)

	assert.Nil(t, tree)
}

func TestExtractCommandTreeExcludesDeprecated(t *testing.T) {
	root := newCmd("myapp", "")
	active := newCmd("active", "Active command")
	deprecated := newCmd("old", "Old command")
	deprecated.Deprecated = "use 'active' instead"
	root.AddCommand(active, deprecated)

	tree := ExtractCommandTree(root)

	require.Equal(t, 1, len(tree))
	assert.Equal(t, "active", tree[0].Name)
}

func TestExtractCommandTreeDeeplyNested(t *testing.T) {
	root := newCmd("myapp", "")
	l1 := newCmd("resources", "Resource commands")
	l2 := newCmd("events", "Event commands")
	l3 := newCmd("list", "List events")
	l2.AddCommand(l3)
	l1.AddCommand(l2)
	root.AddCommand(l1)

	tree := ExtractCommandTree(root)

	require.Equal(t, 1, len(tree))
	require.Equal(t, 1, len(tree[0].Commands))
	require.Equal(t, 1, len(tree[0].Commands[0].Commands))
	assert.Equal(t, "list", tree[0].Commands[0].Commands[0].Name)
	assert.Nil(t, tree[0].Commands[0].Commands[0].Commands)
}

func TestExtractCommandTreeEmptyDescription(t *testing.T) {
	root := newCmd("myapp", "")
	noDesc := newCmd("nodesc", "")
	root.AddCommand(noDesc)

	tree := ExtractCommandTree(root)

	require.Equal(t, 1, len(tree))
	assert.Equal(t, "nodesc", tree[0].Name)
	assert.Equal(t, "", tree[0].Desc)
}
