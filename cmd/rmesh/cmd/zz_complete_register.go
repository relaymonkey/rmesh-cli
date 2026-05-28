package cmd

import (
	"github.com/relaymonkey/relaymesh-edge/internal/clicomplete"
)

func init() {
	// Network references (UUID).
	clicomplete.RegisterArgs(networkUseCmd, clicomplete.NetworksProvider)
	clicomplete.RegisterFlag(trafficCmd, "network", clicomplete.NetworksProvider)

	// Shared output format flags.
	clicomplete.RegisterFlag(networkCmd, "output", clicomplete.StaticStrings("table", "json", "yaml", "id"))
	clicomplete.RegisterFlag(trafficCmd, "output", clicomplete.StaticStrings("table", "json", "yaml", "id"))

	// --fields from GET /messages/fields (Traffic catalog). Persistent on
	// trafficCmd — child commands inherit the same Flag pointer.
	clicomplete.RegisterFlag(trafficCmd, "fields", clicomplete.MessageFieldsProvider)

	// `rmesh device config` source / destination grammar.
	clicomplete.RegisterFlagDirective(deviceConfigShowCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))
	clicomplete.RegisterFlag(deviceConfigShowCmd, "output", clicomplete.StaticStrings("tree", "json", "yaml"))
	clicomplete.RegisterFlag(deviceConfigShowCmd, "section", clicomplete.SectionKeysProvider)

	clicomplete.RegisterFlagDirective(deviceConfigCopyCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))
	clicomplete.RegisterFlagDirective(deviceConfigCopyCmd, "to", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointDestCopy))
	clicomplete.RegisterFlag(deviceConfigCopyCmd, "output", clicomplete.StaticStrings("yaml", "json", "tree"))
	clicomplete.RegisterFlag(deviceConfigCopyCmd, "section", clicomplete.SectionKeysProvider)

	// Deprecated aliases — keep their narrower completion sets so
	// the deprecation guidance matches what tab-complete suggests.
	clicomplete.RegisterFlagDirective(deviceConfigGetCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))
	clicomplete.RegisterFlagDirective(deviceConfigGetCmd, "to", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointDestGet))
	clicomplete.RegisterFlag(deviceConfigGetCmd, "output", clicomplete.StaticStrings("tree", "json", "yaml"))
	clicomplete.RegisterFlag(deviceConfigGetCmd, "section", clicomplete.SectionKeysProvider)

	clicomplete.RegisterFlagDirective(deviceConfigSetCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))
	clicomplete.RegisterFlagDirective(deviceConfigSetCmd, "to", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointDestSet))
	clicomplete.RegisterFlag(deviceConfigSetCmd, "section", clicomplete.SectionKeysProvider)

	clicomplete.RegisterFlag(deviceConfigListCmd, "network", clicomplete.NetworksProvider)
	clicomplete.RegisterFlag(deviceConfigListCmd, "output", clicomplete.StaticStrings("table", "json", "yaml"))

	clicomplete.RegisterFlagDirective(deviceConfigEditCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))
	clicomplete.RegisterFlag(deviceConfigEditCmd, "format", clicomplete.StaticStrings("yaml", "json"))

	clicomplete.RegisterFlagDirective(deviceConfigDeleteCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))
	clicomplete.RegisterFlagDirective(deviceConfigPromoteCmd, "from", clicomplete.ConfigEndpointProvider(clicomplete.ConfigEndpointSource))

	// Agent config overrides — now scoped per verb (see D-220).
	// `pair` deliberately has no transport-url flag.
	clicomplete.RegisterFlagDirective(runCmd, "transport-url", clicomplete.TransportURLProvider)
	clicomplete.RegisterFlagDirective(observeCmd, "transport-url", clicomplete.TransportURLProvider)
	clicomplete.RegisterFlagDirective(doctorCmd, "transport-url", clicomplete.TransportURLProvider)
}
