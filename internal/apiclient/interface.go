package apiclient

import "context"

// CloudClient is the RelayMesh REST surface used by rmesh cloud subcommands.
type CloudClient interface {
	GetMe(ctx context.Context) (Me, error)
	ListNetworks(ctx context.Context) (NetworkList, error)
	ListMessages(ctx context.Context, networkID string, q ListMessagesQuery) (MessageList, error)
	ListMessageFields(ctx context.Context, networkID, from, to string) (MessageFieldCatalog, error)
	StreamLive(ctx context.Context, networkID string, onHello func(map[string]any), onMessage func(MessageEnvelope)) error
	ResolveNetworkRef(ctx context.Context, ref string) (Network, error)
}

var _ CloudClient = (*Client)(nil)
