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

	// --fields from GET /messages/fields (Traffic catalog).
	clicomplete.RegisterFlag(trafficCmd, "fields", clicomplete.MessageFieldsProvider)
	clicomplete.RegisterFlag(trafficLiveCmd, "fields", clicomplete.MessageFieldsProvider)
	clicomplete.RegisterFlag(trafficTextLiveCmd, "fields", clicomplete.MessageFieldsProvider)
}
