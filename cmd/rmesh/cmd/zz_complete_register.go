package cmd

import (
	"github.com/relaymonkey/relaymesh-edge/internal/clicomplete"
)

func init() {
	// Network references (slug, short_id, uuid, name).
	clicomplete.RegisterArgs(networkUseCmd, clicomplete.NetworksProvider)
	clicomplete.RegisterFlag(trafficCmd, "network", clicomplete.NetworksProvider)

	// Shared output format flags.
	clicomplete.RegisterFlag(networkCmd, "output", clicomplete.StaticStrings("table", "json", "yaml", "id"))
	clicomplete.RegisterFlag(trafficCmd, "output", clicomplete.StaticStrings("table", "json", "yaml", "id"))

	// --fields from GET /messages/fields (Traffic catalog). Persistent on
	// trafficCmd — child commands inherit the same Flag pointer.
	clicomplete.RegisterFlag(trafficCmd, "fields", clicomplete.MessageFieldsProvider)

	// `rmesh device config` source / destination grammar (D-209).
	clicomplete.RegisterFlagDirective(deviceConfigGetCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))
	clicomplete.RegisterFlagDirective(deviceConfigGetCmd, "to", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointDestGet))
	clicomplete.RegisterFlag(deviceConfigGetCmd, "output", clicomplete.StaticStrings("tree", "json", "yaml"))
	clicomplete.RegisterFlag(deviceConfigGetCmd, "section", clicomplete.SectionKeysProvider)

	clicomplete.RegisterFlagDirective(deviceConfigSetCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))
	clicomplete.RegisterFlagDirective(deviceConfigSetCmd, "to", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointDestSet))
	clicomplete.RegisterFlag(deviceConfigSetCmd, "section", clicomplete.SectionKeysProvider)

	clicomplete.RegisterFlag(deviceConfigListCmd, "network", clicomplete.NetworksProvider)
	clicomplete.RegisterFlag(deviceConfigListCmd, "output", clicomplete.StaticStrings("table", "json", "yaml"))
}
