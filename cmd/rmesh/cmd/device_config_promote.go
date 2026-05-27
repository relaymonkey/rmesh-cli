package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
)

// promoteFlags is the dedicated `promote` flag block. Kept separate
// from commonFlags so the verb's surface (the things an operator
// types) stays small and obvious in `--help`.
var promoteFlags struct {
	from         string
	label        string
	description  string
	visibility   string
	audienceTags []string
	sections     []string
	allSections  bool
}

// deviceConfigPromoteCmd publishes a personal device config as a new
// network template on the same network.
//
// Why a dedicated verb (and not `set --shared`):
//
//   - Promotion is a state transition (a row's `owner_kind` changes
//     from `user` to `network`), not a transport. Different category
//     of action ⇒ different verb (3-S Signatures axiom).
//   - Reading shell history "saw `promote` on line 47" is unambiguous;
//     "saw `set --shared`" is one missed flag away from a footgun.
//   - The audit action `network.device_config.promoted` maps 1:1 to
//     this verb instead of a flag combination, simplifying audit
//     log search.
var deviceConfigPromoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Publish a personal device config as a network template",
	Long: `Create a new network template on the same network as a
personal device config. The personal source row is left
intact; a fresh template row is created with a separate id.

Section allowlist: the server applies the allowlist server-side.
Default allows the safe template subset (channels, lora, modules,
position, power, network, device, display) and DROPS personal
sections (owner, fixed_position, config.bluetooth, config.security).
Pass --section to override; pass --all-sections to publish every
section that the source carries.

Examples:

  rmesh device config promote --from cloud:home/mine/eu-868 \
      --label eu-868-default

  rmesh device config promote --from cloud:home/mine/parking-gw \
      --label parking-gw --visibility public --audience field-team

  rmesh device config promote --from cloud:home/mine/full-backup \
      --label demo --section channels,config.lora,config.bluetooth`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		if strings.TrimSpace(promoteFlags.label) == "" {
			return errors.New("--label is required")
		}
		if strings.TrimSpace(promoteFlags.from) == "" {
			return errors.New("--from is required (e.g. cloud:home/mine/eu-868)")
		}

		// Parse the source URI. Promote only accepts personal sources;
		// we require the explicit `cloud:<n>/mine/<label>` form (or
		// the bare `cloud:<n>/<label>` shape, which we then resolve
		// against the personal library only). Templates can't be
		// promoted from — they're already templates.
		// Use the existing source parser via the public package.
		_, client, err := requireSession()
		if err != nil {
			return err
		}
		c := concrete(client)
		if c == nil {
			return errors.New("promote requires the concrete *apiclient.Client")
		}

		src, err := parsePromoteSource(promoteFlags.from)
		if err != nil {
			return err
		}

		netID, err := resolveCloudNetworkID(ctx, client, src.network)
		if err != nil {
			return err
		}
		// Resolve the source label against the personal library only.
		// `cloud:<n>/template/<label>` is rejected at parse-time
		// (parsePromoteSource); a bare `cloud:<n>/<label>` is also
		// resolved as personal, so promoting a template by label is
		// not possible by accident.
		cfgID, err := c.ResolveDeviceConfigID(ctx, netID, src.label, apiclient.OwnerMine)
		if err != nil {
			return err
		}

		visibility := strings.TrimSpace(promoteFlags.visibility)
		if visibility == "" {
			visibility = "members"
		}

		var sections []string
		if !promoteFlags.allSections {
			// Empty stays empty here so the server applies its
			// `TemplateSafeSections` default. An operator who
			// wants the full set passes --all-sections.
			sections = promoteFlags.sections
		}
		if promoteFlags.allSections && len(promoteFlags.sections) > 0 {
			return errors.New("--section and --all-sections are mutually exclusive")
		}

		out, err := c.PromoteDeviceConfig(ctx, netID, cfgID, apiclient.PromoteRequest{
			Label:           promoteFlags.label,
			Description:     promoteFlags.description,
			Visibility:      visibility,
			AudienceTags:    promoteFlags.audienceTags,
			IncludeSections: sections,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(),
			"promoted cloud:%s/mine/%s → cloud:%s/template/%s (id=%s, visibility=%s)\n",
			src.network, src.label, src.network, out.Label, out.ID, out.Visibility,
		)
		return nil
	},
}

type promoteSource struct {
	network string
	label   string
}

// parsePromoteSource accepts the same `cloud:<n>/<label>` /
// `cloud:<n>/mine/<label>` shapes ParseSource handles, but rejects
// any other source kind (device / file / template) with a clear
// error. Promote is personal-source-only.
func parsePromoteSource(raw string) (promoteSource, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "cloud:") {
		return promoteSource{}, fmt.Errorf("--from must be a cloud source (got %q); promote publishes personal cloud rows as templates", raw)
	}
	body := s[len("cloud:"):]
	if body == "" {
		return promoteSource{}, errors.New("--from cloud:<network>/<label> required")
	}
	sep := strings.Index(body, "/")
	if sep < 0 {
		return promoteSource{}, fmt.Errorf("cloud source %q missing /<label>", raw)
	}
	net := strings.TrimSpace(body[:sep])
	rest := strings.TrimSpace(body[sep+1:])
	if net == "" || rest == "" {
		return promoteSource{}, fmt.Errorf("cloud source %q must be cloud:<network>/<label>", raw)
	}
	if sep2 := strings.Index(rest, "/"); sep2 > 0 {
		head := strings.TrimSpace(rest[:sep2])
		tail := strings.TrimSpace(rest[sep2+1:])
		switch head {
		case "mine":
			rest = tail
		case "template", "templates", "shared":
			return promoteSource{}, fmt.Errorf(
				"cannot promote %q: source is already a template", raw)
		default:
			// Unknown owner segment — fall through to treat the
			// whole `head/tail` as the label, matching the
			// permissive behaviour of ParseSource.
		}
	}
	if rest == "" {
		return promoteSource{}, fmt.Errorf("cloud source %q must end with a label", raw)
	}
	return promoteSource{network: net, label: rest}, nil
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigPromoteCmd)
	f := deviceConfigPromoteCmd.Flags()
	f.StringVar(&promoteFlags.from, "from", "", "Personal cloud source: cloud:<network>/<label> (or cloud:<network>/mine/<label>)")
	f.StringVar(&promoteFlags.label, "label", "", "Label for the new network template (required)")
	f.StringVar(&promoteFlags.description, "description", "", "Optional description stored on the template")
	f.StringVar(&promoteFlags.visibility, "visibility", "members", "Template visibility: members | public (public requires network:admin)")
	f.StringSliceVar(&promoteFlags.audienceTags, "audience", nil, "Operator-defined audience tags (OQ-DEVCFG-04, metadata only in v1)")
	f.StringSliceVar(&promoteFlags.sections, "section", nil, "Sections to publish (default: server's TemplateSafeSections — drops personal-by-default)")
	f.BoolVar(&promoteFlags.allSections, "all-sections", false, "Publish every section the source carries (overrides --section; intentionally publishes personal sections)")
	_ = deviceConfigPromoteCmd.MarkFlagRequired("from")
	_ = deviceConfigPromoteCmd.MarkFlagRequired("label")
}
