package bootstrap

import (
	"strings"

	"github.com/spf13/cobra"
)

// CommandInfo describes a plugin subcommand for inclusion in the plugins.toml
// manifest. The CLI reads this metadata to display plugin subcommands in
// --map output and shell completion without launching the plugin binary.
type CommandInfo struct {
	Name     string        `toml:"Name" json:"name"`
	Desc     string        `toml:"Desc" json:"desc,omitempty"`
	Commands []CommandInfo `toml:"Command,omitempty" json:"commands,omitempty"`
}

// ExtractCommandTree walks a cobra.Command tree and returns a slice of
// CommandInfo suitable for serialization into a plugins.toml manifest.
// Hidden commands, deprecated commands, and the auto-generated "help"
// command are excluded.
func ExtractCommandTree(cmd *cobra.Command) []CommandInfo {
	var result []CommandInfo
	for _, child := range cmd.Commands() {
		if child.Hidden || len(child.Deprecated) > 0 || strings.EqualFold(child.Name(), "help") {
			continue
		}
		info := CommandInfo{
			Name:     child.Name(),
			Desc:     child.Short,
			Commands: ExtractCommandTree(child),
		}
		if len(info.Commands) == 0 {
			info.Commands = nil
		}
		result = append(result, info)
	}
	return result
}
