package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clioutput"
)

var networkListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List networks you can access",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNetworkList(cmd)
	},
}

func runNetworkList(cmd *cobra.Command) error {
	_, client, err := requireSession()
	if err != nil {
		return err
	}

	format, err := clioutput.ParseFormat(networkOutput)
	if err != nil {
		return err
	}

	list, err := client.ListNetworks(context.Background())
	if err != nil {
		return err
	}

	return clioutput.Render(
		cmd.OutOrStdout(),
		format,
		networkListTable(list.Items),
		list,
		networkListIDs(list.Items),
	)
}

func networkListTable(items []apiclient.Network) clioutput.Table {
	table := clioutput.Table{
		Headers: []string{"NAME", "SLUG", "VISIBILITY", "SHORT_ID", "ID"},
	}
	for _, n := range items {
		table.Rows = append(table.Rows, []string{
			n.Name,
			n.Slug,
			n.Visibility,
			n.ShortID,
			n.ID,
		})
	}
	return table
}

func networkListIDs(items []apiclient.Network) []string {
	ids := make([]string, len(items))
	for i, n := range items {
		ids[i] = n.ID
	}
	return ids
}
