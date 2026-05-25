package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/clitraffic"
	"github.com/relaymonkey/relaymesh-edge/internal/clioutput"
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

	out, err := clitraffic.List(context.Background(), client, networkID, clitraffic.ListInput{
		TextOnly:       textOnly,
		From:           trafficFrom,
		To:             trafficTo,
		Q:              trafficQ,
		Filters:        trafficFilters,
		NodeFilters:    trafficNodeFilters,
		GatewayFilters: trafficGatewayFilters,
		Limit:          trafficLimit,
		FieldsRaw:      trafficFields,
	}, format)
	if err != nil {
		return err
	}

	return clioutput.Render(cmd.OutOrStdout(), format, out.Table, out.Raw, out.IDs)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return clitraffic.Live(ctx, client, networkID, clitraffic.LiveInput{
		TextOnly:  textOnly,
		FieldsRaw: trafficFields,
		Output:    trafficOutput,
	}, cmd.OutOrStdout(), cmd.ErrOrStderr())
}
