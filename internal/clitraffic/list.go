package clitraffic

import (
	"context"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/climessages"
	"github.com/relaymonkey/relaymesh-edge/internal/clioutput"
)

// ListInput holds historical traffic query options (after flags are parsed).
type ListInput struct {
	TextOnly       bool
	From           string
	To             string
	Q              string
	Filters        []string
	NodeFilters    []string
	GatewayFilters []string
	Limit          int
	FieldsRaw      string
}

// ListOutput is ready for clioutput.Render.
type ListOutput struct {
	Table clioutput.Table
	Raw   any
	IDs   []string
}

// List queries historical traffic and projects rows for the selected output format.
func List(ctx context.Context, client apiclient.CloudClient, networkID string, in ListInput, format clioutput.Format) (ListOutput, error) {
	fields, err := climessages.ParseFields(in.FieldsRaw, in.TextOnly)
	if err != nil {
		return ListOutput{}, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = climessages.DefaultLimit
	}

	filters := append([]string(nil), in.Filters...)
	if in.TextOnly {
		filters = append([]string{climessages.TextFilter}, filters...)
	}

	list, err := client.ListMessages(ctx, networkID, apiclient.ListMessagesQuery{
		From:           in.From,
		To:             in.To,
		Q:              in.Q,
		Limit:          limit,
		Filters:        filters,
		NodeFilters:    append([]string(nil), in.NodeFilters...),
		GatewayFilters: append([]string(nil), in.GatewayFilters...),
	})
	if err != nil {
		return ListOutput{}, err
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

	return ListOutput{Table: table, Raw: raw, IDs: ids}, nil
}
