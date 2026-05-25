package clitraffic

import (
	"context"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
)

type fakeCloudClient struct {
	list      apiclient.MessageList
	stream    func(context.Context, string, func(map[string]any), func(apiclient.MessageEnvelope)) error
	resolve   func(context.Context, string) (apiclient.Network, error)
}

func (f *fakeCloudClient) GetMe(context.Context) (apiclient.Me, error) {
	return apiclient.Me{}, nil
}

func (f *fakeCloudClient) ListNetworks(context.Context) (apiclient.NetworkList, error) {
	return apiclient.NetworkList{}, nil
}

func (f *fakeCloudClient) ListMessages(_ context.Context, _ string, _ apiclient.ListMessagesQuery) (apiclient.MessageList, error) {
	return f.list, nil
}

func (f *fakeCloudClient) ListMessageFields(context.Context, string, string, string) (apiclient.MessageFieldCatalog, error) {
	return apiclient.MessageFieldCatalog{}, nil
}

func (f *fakeCloudClient) StreamLive(ctx context.Context, networkID string, hello func(map[string]any), onMsg func(apiclient.MessageEnvelope)) error {
	if f.stream != nil {
		return f.stream(ctx, networkID, hello, onMsg)
	}
	return nil
}

func (f *fakeCloudClient) ResolveNetworkRef(ctx context.Context, ref string) (apiclient.Network, error) {
	if f.resolve != nil {
		return f.resolve(ctx, ref)
	}
	return apiclient.Network{}, nil
}

var _ apiclient.CloudClient = (*fakeCloudClient)(nil)
