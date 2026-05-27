package cmd

import (
	"github.com/spf13/cobra"
)

var (
	trafficNetwork        string
	trafficOutput         string
	trafficLimit          int
	trafficFields         string
	trafficFilters        []string
	trafficNodeFilters    []string
	trafficGatewayFilters []string
	trafficFrom           string
	trafficTo             string
	trafficQ              string
)

var trafficCmd = &cobra.Command{
	Use:     "traffic",
	Aliases: []string{"messages", "msg"},
	Short:   "Query and stream network traffic",
	Long:    "Historical queries use GET /api/v1/networks/{id}/messages. Filters and column ids match the Traffic UI verbatim.",
}

func init() {
	rootCmd.AddCommand(trafficCmd)

	trafficCmd.PersistentFlags().StringVarP(&trafficNetwork, "network", "n", "", "Network UUID (default: saved via network use)")
	trafficCmd.PersistentFlags().StringVarP(&trafficOutput, "output", "o", "table", "Output format: table, json, yaml, id")
	trafficCmd.PersistentFlags().IntVarP(&trafficLimit, "limit", "l", 0, "Max rows (default: API default 100)")
	trafficCmd.PersistentFlags().StringVar(&trafficFields, "fields", "", "Traffic UI column ids (comma-separated; any path on the envelope JSON)")
	trafficCmd.PersistentFlags().StringVar(&trafficFrom, "from", "", "Time range start (RFC3339, ingest_ts lower bound)")
	trafficCmd.PersistentFlags().StringVar(&trafficTo, "to", "", "Time range end (RFC3339, ingest_ts upper bound)")
	trafficCmd.PersistentFlags().StringVar(&trafficQ, "q", "", "Free-text search (same as Traffic search box)")
}
