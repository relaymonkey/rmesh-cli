package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/relaymonkey/relaymesh-edge/internal/apiclient"
	"github.com/relaymonkey/relaymesh-edge/internal/cliui"
	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

// promoteFlags is the dedicated `promote` flag block. Kept separate
// from commonFlags so the verb's surface (the things an operator
// types) stays small and obvious in `--help`.
var promoteFlags struct {
	from         string
	to           string // target network (slug / uuid / short_id) per D-219
	label        string
	description  string
	visibility   string
	audienceTags []string
	sections     []string
	allSections  bool
	edit         bool
	noEdit       bool
}

// templateSafeSectionsCLI mirrors the backend's TemplateSafeSections
// (D-213). The list is duplicated here intentionally so the
// edit-buffer preview shows the same defaults the server would
// apply when --section is unset; keep this in lockstep with
// `internal/deviceconfigs/deviceconfig.go` in relaymesh-backend.
var templateSafeSectionsCLI = []string{
	"channels",
	"config.lora",
	"config.position",
	"config.power",
	"config.network",
	"config.device",
	"config.display",
	"module_config.mqtt",
	"module_config.telemetry",
	"module_config.neighbor_info",
	"module_config.range_test",
	"module_config.store_forward",
	"module_config.serial",
	"module_config.canned_message",
	"module_config.detection_sensor",
	"module_config.paxcounter",
	"module_config.ambient_lighting",
	"module_config.ext_notification",
	"module_config.audio",
	"module_config.remote_hardware",
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

Interactive preview: on a TTY, the staged payload opens in $EDITOR
before the promote request fires (mirrors the UI's promote dialog).
Delete top-level keys you don't want to publish; the remaining
sections become the include_sections list. Field-level edits are
ignored — Promote always uses the source payload, gated by the
sections that remain. Pass --no-edit to skip the editor (the default
on non-TTY shells, e.g. CI). Pass --edit to force it on.

Examples:

  rmesh device config promote --from cloud:mine/eu-868 \
      --to home --label eu-868-default

  rmesh device config promote --from cloud:mine/parking-gw \
      --to parking --label parking-gw --visibility public --audience field-team

  rmesh device config promote --from cloud:mine/full-backup \
      --to home --label demo --section channels,config.lora,config.bluetooth

  rmesh device config promote --from cloud:mine/eu-868 \
      --to home --label eu-868 --no-edit`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		if strings.TrimSpace(promoteFlags.label) == "" {
			return errors.New("--label is required")
		}
		if strings.TrimSpace(promoteFlags.from) == "" {
			return errors.New("--from is required (e.g. cloud:mine/eu-868)")
		}
		if strings.TrimSpace(promoteFlags.to) == "" {
			return errors.New("--to is required: the target network to publish the template onto")
		}
		if promoteFlags.edit && promoteFlags.noEdit {
			return errors.New("--edit and --no-edit are mutually exclusive")
		}
		if promoteFlags.allSections && len(promoteFlags.sections) > 0 {
			return errors.New("--section and --all-sections are mutually exclusive")
		}

		_, client, err := requireSession()
		if err != nil {
			return err
		}
		c := concrete(client)
		if c == nil {
			return errors.New("promote requires the concrete *apiclient.Client")
		}

		srcLabel, err := parsePromoteSource(promoteFlags.from)
		if err != nil {
			return err
		}

		// D-219: target network comes from --to; source is mine.
		targetNetID, err := resolveCloudNetworkID(ctx, client, promoteFlags.to)
		if err != nil {
			return err
		}
		cfgID, err := c.ResolveDeviceConfigID(ctx, "", srcLabel, apiclient.OwnerMine)
		if err != nil {
			return err
		}

		visibility := strings.TrimSpace(promoteFlags.visibility)
		if visibility == "" {
			visibility = "members"
		}

		// Seed include_sections from the flags:
		//   --all-sections  → every section present on the source
		//   --section a,b   → exactly those
		//   (neither)       → TemplateSafeSections
		// The editor (if it opens) further narrows this set.
		var seedSections []string
		switch {
		case promoteFlags.allSections:
			seedSections = nil // sentinel; resolved after we have the payload
		case len(promoteFlags.sections) > 0:
			seedSections = promoteFlags.sections
		default:
			seedSections = templateSafeSectionsCLI
		}

		shouldEdit := resolvePromoteEdit(promoteFlags.edit, promoteFlags.noEdit)

		var includeSections []string
		if shouldEdit {
			// Fetch the source payload with secrets revealed so the
			// operator sees the actual JSON they'd be publishing.
			// The personal row is always readable in the clear by
			// its owner; the cloud surfaces a 403 otherwise.
			srcURI := deviceconfigs.Source{
				Kind:  deviceconfigs.SourceCloud,
				Owner: deviceconfigs.CloudOwnerMine,
				Label: srcLabel,
			}
			payload, _, err := readPayloadFromSource(ctx, srcURI, client, true)
			if err != nil {
				return err
			}
			effectiveSeed := seedSections
			if promoteFlags.allSections {
				effectiveSeed = allPresentSections(payload)
			}
			includeSections, err = runPromoteEditor(payload, effectiveSeed)
			if err != nil {
				return err
			}
			if len(includeSections) == 0 {
				return errors.New("no sections selected; nothing to publish (re-run without --no-edit to keep editing)")
			}
		} else {
			// Non-interactive path: defer to the existing flag
			// semantics. Empty stays empty so the server applies
			// TemplateSafeSections by default; --section / --all-sections
			// override below.
			switch {
			case promoteFlags.allSections:
				includeSections = nil // server treats nil-with-flag... but Promote uses TemplateSafe on empty.
				// To mirror the editor path, expand --all-sections client-side too.
				payload, _, err := readPayloadFromSource(ctx, deviceconfigs.Source{
					Kind:  deviceconfigs.SourceCloud,
					Owner: deviceconfigs.CloudOwnerMine,
					Label: srcLabel,
				}, client, true)
				if err != nil {
					return err
				}
				includeSections = allPresentSections(payload)
			case len(promoteFlags.sections) > 0:
				includeSections = promoteFlags.sections
			default:
				includeSections = nil // server default = TemplateSafeSections
			}
		}

		out, err := c.PromoteDeviceConfig(ctx, "", cfgID, apiclient.PromoteRequest{
			TargetNetworkID: targetNetID,
			Label:           promoteFlags.label,
			Description:     promoteFlags.description,
			Visibility:      visibility,
			AudienceTags:    promoteFlags.audienceTags,
			IncludeSections: includeSections,
		})
		if err != nil {
			return err
		}
		fromPath := deviceconfigs.Source{
			Kind:  deviceconfigs.SourceCloud,
			Owner: deviceconfigs.CloudOwnerMine,
			Label: srcLabel,
		}.String()
		toPath := deviceconfigs.Source{
			Kind:    deviceconfigs.SourceCloud,
			Network: promoteFlags.to,
			Owner:   deviceconfigs.CloudOwnerTemplate,
			Label:   out.Label,
		}.String()
		return cliui.New(cmd.OutOrStdout()).PromotedCloudConfig(
			out.Label, fromPath, toPath, out.ID, out.Visibility,
		)
	},
}

// resolvePromoteEdit decides whether to open $EDITOR before the
// promote request fires. The default mirrors the UI's promote
// dialog ("always preview") on a TTY; on a non-TTY shell (CI, piped
// invocation) it stays off so scripted use is unchanged.
func resolvePromoteEdit(edit, noEdit bool) bool {
	if edit {
		return true
	}
	if noEdit {
		return false
	}
	// Stdin TTY is the relevant signal — that's what the editor
	// will inherit. A piped-stdin invocation can't drive an
	// interactive editor.
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// allPresentSections walks a payload and returns every section that
// actually carries content, in canonical-path form. Used both by
// --all-sections (client-side expansion) and as the union the editor
// path uses to derive the final include list.
func allPresentSections(p deviceconfigs.CanonicalPayload) []string {
	out := make([]string, 0, 4+len(p.Config)+len(p.ModuleConfig))
	if len(p.Channels) > 0 {
		out = append(out, "channels")
	}
	if len(p.Owner) > 0 {
		out = append(out, "owner")
	}
	if len(p.FixedPosition) > 0 {
		out = append(out, "fixed_position")
	}
	cfgKeys := make([]string, 0, len(p.Config))
	for k, v := range p.Config {
		if len(v) > 0 {
			cfgKeys = append(cfgKeys, k)
		}
	}
	sort.Strings(cfgKeys)
	for _, k := range cfgKeys {
		out = append(out, "config."+k)
	}
	modKeys := make([]string, 0, len(p.ModuleConfig))
	for k, v := range p.ModuleConfig {
		if len(v) > 0 {
			modKeys = append(modKeys, k)
		}
	}
	sort.Strings(modKeys)
	for _, k := range modKeys {
		out = append(out, "module_config."+k)
	}
	return out
}

// filterPromoteSections returns a copy of `p` restricted to the
// canonical section paths in `sections`. Mirrors the backend's
// `FilterPayloadSections` so the edit preview matches what the
// server would publish for the same include list. Unknown section
// paths are ignored.
func filterPromoteSections(p deviceconfigs.CanonicalPayload, sections []string) deviceconfigs.CanonicalPayload {
	keep := make(map[string]struct{}, len(sections))
	for _, s := range sections {
		keep[s] = struct{}{}
	}
	out := deviceconfigs.CanonicalPayload{}
	if _, ok := keep["channels"]; ok {
		out.Channels = p.Channels
	}
	if _, ok := keep["owner"]; ok {
		out.Owner = p.Owner
	}
	if _, ok := keep["fixed_position"]; ok {
		out.FixedPosition = p.FixedPosition
	}
	for k, v := range p.Config {
		if _, ok := keep["config."+k]; ok {
			if out.Config == nil {
				out.Config = map[string]json.RawMessage{}
			}
			out.Config[k] = v
		}
	}
	for k, v := range p.ModuleConfig {
		if _, ok := keep["module_config."+k]; ok {
			if out.ModuleConfig == nil {
				out.ModuleConfig = map[string]json.RawMessage{}
			}
			out.ModuleConfig[k] = v
		}
	}
	return out
}

// promoteEditorHeader is the comment block prepended to the editor
// buffer. YAML treats `#` lines as comments and the parser drops
// them, so the preamble is purely instructional.
const promoteEditorHeader = `# Edit the staged template payload below.
#
# - Each top-level YAML key is a section that will be published.
# - Delete a section (and its sub-tree) to drop it from the new
#   template. Re-add a section from the personal source by typing
#   its key back in (channels, owner, fixed_position, config.<key>,
#   module_config.<key>).
# - Value edits inside a section are IGNORED — Promote always uses
#   the source row's payload, gated by which sections remain here.
#   To change values, edit the source first with
#   ` + "`rmesh device config edit --from cloud:<n>/mine/<label>`" + ` then
#   re-run promote.
# - Save with no sections left to cancel the promote.
#
# Lines beginning with '#' are ignored.
`

// runPromoteEditor renders the seeded payload to $EDITOR, lets the
// operator trim sections, and returns the resulting include list.
// Returns an empty slice if the operator saved an empty buffer
// (the caller treats this as a cancel).
func runPromoteEditor(payload deviceconfigs.CanonicalPayload, seed []string) ([]string, error) {
	preview := filterPromoteSections(payload, seed)

	tmp, err := os.CreateTemp("", "rmesh-promote-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(promoteEditorHeader); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("write editor header: %w", err)
	}
	if err := deviceconfigs.Render(tmp, preview, deviceconfigs.FormatYAML); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("render payload: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	originalBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read temp file: %w", err)
	}

	if err := runEditor(tmpPath); err != nil {
		return nil, err
	}

	editedBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("re-read temp file: %w", err)
	}
	// Byte-identical buffer ⇒ operator accepted the seed unchanged.
	if bytes.Equal(originalBytes, editedBytes) {
		return append([]string(nil), seed...), nil
	}

	edited, err := deviceconfigs.ParseBytes(editedBytes, tmpPath)
	if err != nil {
		return nil, fmt.Errorf("parse edited payload: %w (re-edit and try again, or discard the buffer)", err)
	}
	return allPresentSections(edited), nil
}

// parsePromoteSource accepts a personal cloud source and returns the
// label. Per D-219 the source is always `cloud:mine/<label>` (no
// network). Legacy `cloud:<n>/mine/<label>` is accepted with the
// network segment ignored. Templates / device / file sources are
// rejected — promote is personal-source-only.
func parsePromoteSource(raw string) (string, error) {
	src, err := deviceconfigs.ParseSource(raw)
	if err != nil {
		return "", fmt.Errorf("--from %q: %w", raw, err)
	}
	if src.Kind != deviceconfigs.SourceCloud {
		return "", fmt.Errorf("--from must be a cloud source (got %q); promote publishes personal cloud rows as templates", raw)
	}
	if src.Owner == deviceconfigs.CloudOwnerTemplate {
		return "", fmt.Errorf("cannot promote %q: source is already a template", raw)
	}
	if src.Owner != deviceconfigs.CloudOwnerMine {
		return "", fmt.Errorf("--from must be a personal source: cloud:mine/<label> (got %q)", raw)
	}
	if strings.TrimSpace(src.Label) == "" {
		return "", fmt.Errorf("cloud source %q must end with a label", raw)
	}
	return src.Label, nil
}

func init() {
	deviceConfigCmd.AddCommand(deviceConfigPromoteCmd)
	f := deviceConfigPromoteCmd.Flags()
	f.StringVar(&promoteFlags.from, "from", "", "Personal cloud source: cloud:mine/<label>")
	f.StringVar(&promoteFlags.to, "to", "", "Target network to publish the template onto (slug / uuid / short_id)")
	f.StringVar(&promoteFlags.label, "label", "", "Label for the new network template (required)")
	f.StringVar(&promoteFlags.description, "description", "", "Optional description stored on the template")
	f.StringVar(&promoteFlags.visibility, "visibility", "members", "Template visibility: members | public (public requires network:admin)")
	f.StringSliceVar(&promoteFlags.audienceTags, "audience", nil, "Operator-defined audience tags (OQ-DEVCFG-04, metadata only in v1)")
	f.StringSliceVar(&promoteFlags.sections, "section", nil, "Sections to publish (default: server's TemplateSafeSections — drops personal-by-default)")
	f.BoolVar(&promoteFlags.allSections, "all-sections", false, "Publish every section the source carries (overrides --section; intentionally publishes personal sections)")
	f.BoolVar(&promoteFlags.edit, "edit", false, "Force-open $EDITOR with the staged payload before promoting (default on TTY)")
	f.BoolVar(&promoteFlags.noEdit, "no-edit", false, "Skip the editor preview and publish using the flags as-is (default off TTY)")
	_ = deviceConfigPromoteCmd.MarkFlagRequired("from")
	_ = deviceConfigPromoteCmd.MarkFlagRequired("to")
	_ = deviceConfigPromoteCmd.MarkFlagRequired("label")
}
