package cmd

import (
	"github.com/spf13/cobra"
)

// deviceCmd is the namespace root for direct-device interaction
// verbs.
//
// We deliberately keep this root **empty of leaf verbs**
// for now and put all configuration I/O under `rmesh device config
// <verb>`. The bare `rmesh device <verb>` namespace is reserved for
// device-management verbs (`list`, `onboard`, `reboot`, `ping`,
// `factory-reset`, `info`) that will land later — naming them now
// would lock in semantics before those features are designed.
//
// Note: this is distinct from `rmesh config` (which edits the
// **agent** YAML) and from `rmesh agent` (which runs the local
// ingest loop).
var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Interact with a local Meshtastic device",
	Long: `Interact with a local Meshtastic device.

Today this group only carries the ` + "`config`" + ` subcommand. The bare
` + "`rmesh device <verb>`" + ` namespace is reserved for future device-
management verbs (list, onboard, reboot, ping, factory-reset, info)
— see the device configuration specs for the namespace claim.`,
}

func init() {
	rootCmd.AddCommand(deviceCmd)
	deviceCmd.AddCommand(deviceConfigCmd)
}
