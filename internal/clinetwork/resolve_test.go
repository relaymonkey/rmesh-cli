package clinetwork

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/clidefault"
)

type stubCloudClient struct {
	resolve func(context.Context, string) (apiclient.Network, error)
}

func (s *stubCloudClient) GetMe(context.Context) (apiclient.Me, error) {
	return apiclient.Me{}, nil
}

func (s *stubCloudClient) ListNetworks(context.Context) (apiclient.NetworkList, error) {
	return apiclient.NetworkList{}, nil
}

func (s *stubCloudClient) ListMessages(context.Context, string, apiclient.ListMessagesQuery) (apiclient.MessageList, error) {
	return apiclient.MessageList{}, nil
}

func (s *stubCloudClient) ListMessageFields(context.Context, string, string, string) (apiclient.MessageFieldCatalog, error) {
	return apiclient.MessageFieldCatalog{}, nil
}

func (s *stubCloudClient) StreamLive(context.Context, string, func(map[string]any), func(apiclient.MessageEnvelope)) error {
	return nil
}

func (s *stubCloudClient) ResolveNetworkRef(ctx context.Context, ref string) (apiclient.Network, error) {
	if s.resolve != nil {
		return s.resolve(ctx, ref)
	}
	return apiclient.Network{}, nil
}

var _ apiclient.CloudClient = (*stubCloudClient)(nil)

func TestResolveIDFromRef(t *testing.T) {
	client := &stubCloudClient{
		resolve: func(_ context.Context, ref string) (apiclient.Network, error) {
			if ref != "norway" {
				t.Fatalf("ref = %q", ref)
			}
			return apiclient.Network{ID: "uuid-norway"}, nil
		},
	}
	id, err := ResolveID(context.Background(), client, "norway")
	if err != nil {
		t.Fatal(err)
	}
	if id != "uuid-norway" {
		t.Fatalf("id = %q", id)
	}
}

func TestResolveIDFromDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RMESH_DEFAULT_NETWORK_FILE", filepath.Join(dir, "default-network.json"))
	if err := clidefault.Save(clidefault.Network{NetworkID: "uuid-default"}); err != nil {
		t.Fatal(err)
	}

	id, err := ResolveID(context.Background(), &stubCloudClient{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if id != "uuid-default" {
		t.Fatalf("id = %q", id)
	}
}

func TestResolveIDNoRefOrDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RMESH_DEFAULT_NETWORK_FILE", filepath.Join(dir, "default-network.json"))
	if err := clidefault.Clear(); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveID(context.Background(), &stubCloudClient{}, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no network specified") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveIDPropagateResolveError(t *testing.T) {
	want := fmt.Errorf("network %q not found", "missing")
	client := &stubCloudClient{
		resolve: func(_ context.Context, _ string) (apiclient.Network, error) {
			return apiclient.Network{}, want
		},
	}
	_, err := ResolveID(context.Background(), client, "missing")
	if !errors.Is(err, want) && err.Error() != want.Error() {
		t.Fatalf("err = %v want %v", err, want)
	}
}
