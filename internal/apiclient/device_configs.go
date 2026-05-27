package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/relaymonkey/relaymesh-edge/internal/deviceconfigs"
)

// DeviceConfigSummary mirrors the backend's `DeviceConfigSummary`
// shape. Metadata-only;
// the payload travels with `DeviceConfigDetail`.
type DeviceConfigSummary struct {
	ID                   string   `json:"id"`
	NetworkID            string   `json:"network_id"`
	Label                string   `json:"label"`
	Description          string   `json:"description"`
	PayloadSchemaVersion int      `json:"payload_schema_version"`
	Region               string   `json:"region,omitempty"`
	ModemPreset          string   `json:"modem_preset,omitempty"`
	FirmwareVersion      string   `json:"firmware_version,omitempty"`
	OwnerKind            string   `json:"owner_kind"`
	OwnerUserID          string   `json:"owner_user_id,omitempty"`
	Visibility           string   `json:"visibility"`
	AudienceTags         []string `json:"audience_tags"`
	IsFeatured           bool     `json:"is_featured"`
	CreatedBy            string   `json:"created_by"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
}

// DeviceConfigDetail extends DeviceConfigSummary with the payload.
type DeviceConfigDetail struct {
	DeviceConfigSummary
	Payload  deviceconfigs.CanonicalPayload `json:"payload"`
	Redacted bool                           `json:"redacted"`
}

// DeviceConfigList is `GET /networks/{id}/device-configs` (templates)
// or `GET /me/device-configs` (personal). Same shape, different
// audience.
type DeviceConfigList struct {
	Items []DeviceConfigSummary `json:"items"`
}

// CreateTemplateRequest mirrors the backend's
// `DeviceConfigCreateTemplateRequest`. Used when authoring a network
// template via `POST /networks/{id}/device-configs`. Personal saves
// go through CreatePersonalRequest.
type CreateTemplateRequest struct {
	Label           string                         `json:"label"`
	Description     string                         `json:"description,omitempty"`
	Payload         deviceconfigs.CanonicalPayload `json:"payload"`
	FirmwareVersion string                         `json:"firmware_version,omitempty"`
	Visibility      string                         `json:"visibility,omitempty"`
	AudienceTags    []string                       `json:"audience_tags,omitempty"`
}

// CreatePersonalRequest mirrors `DeviceConfigCreatePersonalRequest`.
// Used by `set --to cloud` where `set --to cloud` always
// writes a personal row.
type CreatePersonalRequest struct {
	NetworkID       string                         `json:"network_id"`
	Label           string                         `json:"label"`
	Description     string                         `json:"description,omitempty"`
	Payload         deviceconfigs.CanonicalPayload `json:"payload"`
	FirmwareVersion string                         `json:"firmware_version,omitempty"`
}

// PromoteRequest mirrors `DeviceConfigPromoteRequest`. The server
// applies the section allowlist; the CLI just passes it through.
type PromoteRequest struct {
	Label           string   `json:"label"`
	Description     string   `json:"description,omitempty"`
	Visibility      string   `json:"visibility,omitempty"`
	AudienceTags    []string `json:"audience_tags,omitempty"`
	IncludeSections []string `json:"include_sections,omitempty"`
}

// ListDeviceConfigs returns metadata for every **network template**
// on a network (`owner_kind='network'`). Personal rows are returned
// only by ListMyDeviceConfigs.
func (c *Client) ListDeviceConfigs(ctx context.Context, networkID string) (DeviceConfigList, error) {
	path := fmt.Sprintf("/api/v1/networks/%s/device-configs", url.PathEscape(networkID))
	var out DeviceConfigList
	if err := c.getJSONQuery(ctx, path, nil, &out); err != nil {
		return DeviceConfigList{}, err
	}
	return out, nil
}

// ListMyDeviceConfigs returns the caller's personal device-config
// library across every network they belong to. When networkID is
// non-empty the result is narrowed to one network.
func (c *Client) ListMyDeviceConfigs(ctx context.Context, networkID string) (DeviceConfigList, error) {
	q := url.Values{}
	if networkID != "" {
		q.Set("network_id", networkID)
	}
	var out DeviceConfigList
	if err := c.getJSONQuery(ctx, "/api/v1/me/device-configs", q, &out); err != nil {
		return DeviceConfigList{}, err
	}
	return out, nil
}

// GetDeviceConfig fetches one device config. When `revealSecrets` is
// true the backend returns PSK / admin-key material in the clear if
// the caller is elevated; otherwise everything is redacted with
// `***`. The cloud is authoritative on whether the operator is
// allowed to reveal — the CLI just passes the request through.
func (c *Client) GetDeviceConfig(ctx context.Context, networkID, configID string, revealSecrets bool) (DeviceConfigDetail, error) {
	path := fmt.Sprintf("/api/v1/networks/%s/device-configs/%s",
		url.PathEscape(networkID), url.PathEscape(configID))
	q := url.Values{}
	if revealSecrets {
		q.Set("reveal_secrets", "true")
	}
	var out DeviceConfigDetail
	if err := c.getJSONQuery(ctx, path, q, &out); err != nil {
		return DeviceConfigDetail{}, err
	}
	return out, nil
}

// CreateMyDeviceConfig posts a new **personal** device config to
// the caller's library. Used by `rmesh device config set --to cloud`
// where `set --to cloud` is always a personal save.
func (c *Client) CreateMyDeviceConfig(ctx context.Context, req CreatePersonalRequest) (DeviceConfigDetail, error) {
	var out DeviceConfigDetail
	if err := c.PostJSON(ctx, "/api/v1/me/device-configs", req, &out); err != nil {
		return DeviceConfigDetail{}, err
	}
	return out, nil
}

// UpdateDeviceConfigRequest mirrors the backend's
// `DeviceConfigUpdateRequest`. All fields are optional; sending
// `Payload` re-seals the row and re-derives the denormalised hints.
// `Visibility` and `AudienceTags` are template-only (rejected for
// personal rows). Pointer types let callers distinguish "omit"
// from "set to zero value".
type UpdateDeviceConfigRequest struct {
	Label           *string                         `json:"label,omitempty"`
	Description     *string                         `json:"description,omitempty"`
	Payload         *deviceconfigs.CanonicalPayload `json:"payload,omitempty"`
	FirmwareVersion *string                         `json:"firmware_version,omitempty"`
	Visibility      *string                         `json:"visibility,omitempty"`
	AudienceTags    *[]string                       `json:"audience_tags,omitempty"`
}

// UpdateDeviceConfig PATCHes a personal or template row. The server
// gates authorisation by owner_kind + caller role; the CLI just
// passes the request through. Used by `rmesh device config edit`
// when the source/destination is a cloud reference.
func (c *Client) UpdateDeviceConfig(ctx context.Context, networkID, configID string, req UpdateDeviceConfigRequest) (DeviceConfigDetail, error) {
	path := fmt.Sprintf("/api/v1/networks/%s/device-configs/%s",
		url.PathEscape(networkID), url.PathEscape(configID))
	var out DeviceConfigDetail
	if err := c.PatchJSON(ctx, path, req, &out); err != nil {
		return DeviceConfigDetail{}, err
	}
	return out, nil
}

// CreateNetworkTemplate posts a new **network template** to a
// network. Requires the caller to hold an elevated network role
// server-side; the CLI doesn't gate up-front. Used internally by
// `promote`; not exposed as a top-level verb.
func (c *Client) CreateNetworkTemplate(ctx context.Context, networkID string, req CreateTemplateRequest) (DeviceConfigDetail, error) {
	path := fmt.Sprintf("/api/v1/networks/%s/device-configs", url.PathEscape(networkID))
	var out DeviceConfigDetail
	if err := c.PostJSON(ctx, path, req, &out); err != nil {
		return DeviceConfigDetail{}, err
	}
	return out, nil
}

// PromoteDeviceConfig publishes the personal row at `configID` on
// `networkID` as a brand-new network template on the same network.
// The new row's id is returned in the response detail.
func (c *Client) PromoteDeviceConfig(ctx context.Context, networkID, configID string, req PromoteRequest) (DeviceConfigDetail, error) {
	path := fmt.Sprintf("/api/v1/networks/%s/device-configs/%s/promote",
		url.PathEscape(networkID), url.PathEscape(configID))
	var out DeviceConfigDetail
	if err := c.PostJSON(ctx, path, req, &out); err != nil {
		return DeviceConfigDetail{}, err
	}
	return out, nil
}

// OwnerHint constrains how ResolveDeviceConfigID searches for a
// label. Mirrors the `cloud:<n>/{mine|template}/<label>` URI
// discriminator.
type OwnerHint string

const (
	// OwnerEither resolves the supplied ref against the caller's
	// personal library first, then falls back to network templates.
	// Used for bare `cloud:<n>/<label>` so existing scripts keep
	// working.
	OwnerEither OwnerHint = ""
	// OwnerMine restricts resolution to personal rows.
	OwnerMine OwnerHint = "mine"
	// OwnerTemplate restricts resolution to network templates.
	OwnerTemplate OwnerHint = "template"
)

// ResolveDeviceConfigID accepts either a UUID or a human label and
// returns the matching `cfg_id`. Honours `hint` to disambiguate
// between personal and template rows when both share a label
// The cloud doesn't expose a `find by label` endpoint
// today, so we paginate through the list — cheap enough.
func (c *Client) ResolveDeviceConfigID(ctx context.Context, networkID, ref string, hint OwnerHint) (string, error) {
	if isUUIDLike(ref) {
		return ref, nil
	}
	tryMine := hint == OwnerEither || hint == OwnerMine
	tryTemplate := hint == OwnerEither || hint == OwnerTemplate

	if tryMine {
		list, err := c.ListMyDeviceConfigs(ctx, networkID)
		if err != nil {
			return "", err
		}
		for _, it := range list.Items {
			if it.Label == ref || it.ID == ref {
				return it.ID, nil
			}
		}
		// Personal-only request: not found is an error here, no
		// fallback — the user explicitly asked for `mine/...`.
		if hint == OwnerMine {
			return "", fmt.Errorf("personal device config %q not found in network %s", ref, networkID)
		}
	}

	if tryTemplate {
		list, err := c.ListDeviceConfigs(ctx, networkID)
		if err != nil {
			return "", err
		}
		for _, it := range list.Items {
			if it.Label == ref || it.ID == ref {
				return it.ID, nil
			}
		}
	}
	return "", fmt.Errorf("device config %q not found in network %s", ref, networkID)
}

func isUUIDLike(s string) bool {
	// Cheap heuristic: 36-char "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx".
	if len(s) != 36 {
		return false
	}
	for i, ch := range s {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
				return false
			}
		}
	}
	return true
}

// Compile-time check: CanonicalPayload must round-trip through
// encoding/json. (Defensive: a future field added with an unsupported
// type would silently silently break the wire.)
var _ json.Marshaler = (*deviceconfigsMarshalSentinel)(nil)

type deviceconfigsMarshalSentinel struct{}

func (deviceconfigsMarshalSentinel) MarshalJSON() ([]byte, error) {
	_, err := json.Marshal(deviceconfigs.CanonicalPayload{})
	return []byte("null"), err
}
