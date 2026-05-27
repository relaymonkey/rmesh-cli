package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/config"
	rmdevice "github.com/relaymonkey/relaymesh-edge/internal/device"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
	rmtransport "github.com/relaymonkey/relaymesh-edge/internal/transport"
)

// deviceConfigCmd is the `rmesh device config` namespace root. Carries
// the canonical verbs (show / copy / edit / list / promote). The verb-specific files (device_config_show.go, …)
// attach their commands to this root in their own init() blocks so
// each verb's flag block stays adjacent to its handler.
var deviceConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show / copy / edit / list / promote / delete full-surface device configurations",
	Long: `Read and write the full device-configuration surface
(settings, modules, and channel rows) using a uniform --from / --to source grammar.

Verbs:

  rmesh device config show     --from <src>                       # read → stdout
  rmesh device config copy     --from <src> --to <dst> [--dry-run]  # transfer A → B
  rmesh device config edit     --from <src>                       # $EDITOR round-trip
  rmesh device config list     [--network <n>]
  rmesh device config promote  --from cloud:<n>/mine/<label> --to <network> ...
  rmesh device config delete   --from cloud:<n>/mine|<template>/<label> [--yes]

A source/destination token is one of:

  device                       live device (uses agent config URL)
  device:<transport-url>       live device at an explicit URL
  file:<path> | ./relative.yml local JSON / YAML file
  cloud:<network>/<label-or-id> saved cloud config (requires session)
  -                            stdout (only valid as --to)

Mental model: ` + "`show`" + ` reads, ` + "`copy`" + ` transfers. ` + "`copy`" + ` is
destination-agnostic — file, cloud and device are all valid ` + "`--to`" + `
targets; the side effects (radio reboot, cloud row creation, file
write) live in the per-destination handler. Use ` + "`copy --dry-run`" + ` to
preview a device apply.

There is no separate ` + "`diff`" + ` verb — ` + "`copy --dry-run`" + ` shows
the diff in the natural "device current → intended" direction.

Deprecated aliases (kept for backwards compatibility):
  get → show / copy --to <file>
  set → copy`,
}

// commonFlags holds the union of flags every verb may want. Individual
// verbs only wire up the subset that matters to them.
type commonFlags struct {
	from    string
	to      string
	output  string
	section []string
	reveal  bool

	// set-specific
	dryRun            bool
	exclude           []string
	allowRegionChange bool
	label             string
	description       string
	public            bool
	rebootWait        time.Duration
	verbose           bool

	// list-specific
	network string
}

// resolveDeviceURL returns the transport URL to use for a `device`
// source whose explicit URL field is empty. Falls back to the agent
// config's transport.url.
func resolveDeviceURL(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	path, err := loadConfig()
	if err != nil {
		return "", fmt.Errorf("no --from device:<url> and agent config unavailable: %w", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", fmt.Errorf("load agent config: %w", err)
	}
	if cfg.Transport.URL == "" {
		return "", errors.New("agent config has no transport.url; pass --from device:<url> explicitly")
	}
	return cfg.Transport.URL, nil
}

// readPayloadFromSource loads a canonical payload from any source. The
// fourth return value is `fwHint` — the firmware version, available
// only for `device` sources, used to populate the denormalised hint
// when uploading to the cloud.
func readPayloadFromSource(
	ctx context.Context,
	src deviceconfigs.Source,
	client apiclient.CloudClient,
	revealForCloud bool,
) (deviceconfigs.CanonicalPayload, string, error) {
	switch src.Kind {
	case deviceconfigs.SourceDevice:
		url, err := resolveDeviceURL(src.URL)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, "", err
		}
		transport, err := rmtransport.Open(url)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, "", fmt.Errorf("open transport %s: %w", url, err)
		}
		defer rmtransport.Close(transport)
		readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		state, err := rmdevice.GetState(readCtx, transport)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, "", fmt.Errorf("read device state: %w", err)
		}
		payload, err := rmdevice.ToCanonicalPayload(state)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, "", err
		}
		return payload, state.FirmwareVersion(), nil

	case deviceconfigs.SourceFile:
		payload, err := deviceconfigs.LoadFromFile(src.Path)
		return payload, "", err

	case deviceconfigs.SourceCloud:
		if client == nil {
			return deviceconfigs.CanonicalPayload{}, "", errors.New("cloud source requires --network resolution; run `rmesh auth login` first")
		}
		netID, err := resolveCloudNetworkID(ctx, client, src.Network)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, "", err
		}
		hint := apiclient.OwnerHint(src.Owner)
		cfgID, err := concrete(client).ResolveDeviceConfigID(ctx, netID, src.Label, hint)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, "", err
		}
		detail, err := concrete(client).GetDeviceConfig(ctx, netID, cfgID, revealForCloud)
		if err != nil {
			return deviceconfigs.CanonicalPayload{}, "", err
		}
		return detail.Payload, detail.FirmwareVersion, nil
	}
	return deviceconfigs.CanonicalPayload{}, "", fmt.Errorf("unsupported source kind for read: %s", src)
}

// resolveCloudNetworkID accepts any of {uuid, slug, short_id, name}.
func resolveCloudNetworkID(ctx context.Context, client apiclient.CloudClient, ref string) (string, error) {
	n, err := client.ResolveNetworkRef(ctx, ref)
	if err != nil {
		return "", err
	}
	return n.ID, nil
}

// concrete narrows the CloudClient interface to the concrete *Client,
// which carries the extra device-config methods (PostJSON, the
// device-config helpers). The interface stays narrow for testability
// elsewhere; callers that need the richer surface use this escape
// hatch.
func concrete(client apiclient.CloudClient) *apiclient.Client {
	if c, ok := client.(*apiclient.Client); ok {
		return c
	}
	// Fallback: build a fresh client from the saved session. This
	// only happens when a test passes an alternative implementation.
	return nil
}

// openOutput opens the --to target for write. "-" or "" maps to stdout.
// Returned closeFn is always non-nil; it's a no-op for stdout.
func openOutput(target string) (io.Writer, func() error, error) {
	if target == "" || target == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(target)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s: %w", target, err)
	}
	return f, f.Close, nil
}
