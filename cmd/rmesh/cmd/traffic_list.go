package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clioutput"
	"github.com/relaymonkey/relaymesh-edge/internal/climessages"
)

var trafficListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List historical traffic",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTrafficList(cmd, false)
	},
}

var trafficTextCmd = &cobra.Command{
	Use:   "text",
	Short: "List text messages (packet_type=1)",
	Long:  "Shorthand for list with filter packet_type:eq:1.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTrafficList(cmd, true)
	},
}

var trafficTextLiveCmd = &cobra.Command{
	Use:   "live",
	Short: "Stream live text messages (WebSocket)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTrafficLive(cmd, true)
	},
}

var trafficLiveCmd = &cobra.Command{
	Use:   "live",
	Short: "Stream live traffic (WebSocket)",
	RunE: func(cmd *cobra.Command, args []string) error {
		textOnly, _ := cmd.Flags().GetBool("text")
		return runTrafficLive(cmd, textOnly)
	},
}

func init() {
	trafficCmd.AddCommand(trafficListCmd)
	trafficCmd.AddCommand(trafficTextCmd)
	trafficCmd.AddCommand(trafficLiveCmd)
	trafficTextCmd.AddCommand(trafficTextLiveCmd)

	trafficListCmd.Flags().StringArrayVar(&trafficFilters, "filter", nil, "Structured filter (field:op:value); passed raw to the API")
	trafficListCmd.Flags().StringArrayVar(&trafficNodeFilters, "node-filter", nil, "Node dimension filter (node.<key>:op:value)")
	trafficListCmd.Flags().StringArrayVar(&trafficGatewayFilters, "gateway-filter", nil, "Gateway dimension filter (gateway.<key>:op:value)")

	trafficTextCmd.Flags().StringArrayVar(&trafficFilters, "filter", nil, "Additional filters (AND with packet_type:eq:1)")
	trafficTextCmd.Flags().StringArrayVar(&trafficNodeFilters, "node-filter", nil, "Node dimension filter (node.<key>:op:value)")
	trafficTextCmd.Flags().StringArrayVar(&trafficGatewayFilters, "gateway-filter", nil, "Gateway dimension filter (gateway.<key>:op:value)")

	trafficLiveCmd.Flags().Bool("text", false, "Only print TEXT_MESSAGE_APP (portnum 1)")

	trafficListCmd.SilenceUsage = true
	trafficTextCmd.SilenceUsage = true
	trafficTextLiveCmd.SilenceUsage = true
	trafficLiveCmd.SilenceUsage = true
}

func runTrafficList(cmd *cobra.Command, textOnly bool) error {
	_, client, err := requireSession()
	if err != nil {
		return err
	}
	networkID, err := resolveNetworkID(cmd, client, trafficNetwork)
	if err != nil {
		return err
	}

	format, err := clioutput.ParseFormat(trafficOutput)
	if err != nil {
		return err
	}

	fields, err := climessages.ParseFields(trafficFields, textOnly)
	if err != nil {
		return err
	}

	limit := trafficLimit
	if limit <= 0 {
		limit = climessages.DefaultLimit
	}

	filters := append([]string(nil), trafficFilters...)
	if textOnly {
		filters = append([]string{climessages.TextFilter}, filters...)
	}

	list, err := client.ListMessages(context.Background(), networkID, apiclient.ListMessagesQuery{
		From:           trafficFrom,
		To:             trafficTo,
		Q:              trafficQ,
		Limit:          limit,
		Filters:        filters,
		NodeFilters:    append([]string(nil), trafficNodeFilters...),
		GatewayFilters: append([]string(nil), trafficGatewayFilters...),
	})
	if err != nil {
		return err
	}

	headers, rows := climessages.ProjectTable(list.Items, fields)
	table := clioutput.Table{Headers: headers, Rows: rows}

	ids := make([]string, len(list.Items))
	for i, m := range list.Items {
		ids[i] = m.StringField("id")
	}

	raw := any(list)
	if format == clioutput.FormatJSON || format == clioutput.FormatYAML {
		raw = climessages.ProjectJSON(list.Items, fields)
	}

	return clioutput.Render(cmd.OutOrStdout(), format, table, raw, ids)
}

func runTrafficLive(cmd *cobra.Command, textOnly bool) error {
	_, client, err := requireSession()
	if err != nil {
		return err
	}
	networkID, err := resolveNetworkID(cmd, client, trafficNetwork)
	if err != nil {
		return err
	}

	fields, err := climessages.ParseFields(trafficFields, textOnly)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")

	return client.StreamLive(ctx, networkID,
		func(hello map[string]any) {
			fmt.Fprintf(cmd.ErrOrStderr(), "connected: network=%v server_ts=%v\n", hello["network_id"], hello["server_ts"])
		},
		func(env apiclient.MessageEnvelope) {
			if textOnly && !climessages.IsText(env) {
				return
			}
			out := trafficOutput
			if out == "" {
				out = "table"
			}
			if out == "json" {
				_ = enc.Encode(climessages.ProjectJSON([]apiclient.MessageEnvelope{env}, fields)[0])
				return
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), climessages.FormatLiveLine(env, fields))
		},
	)
}
