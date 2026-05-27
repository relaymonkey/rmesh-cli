package deviceconfigs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SubmessageDiff is one section's worth of differences.
//
// Status is one of:
//   - "added"    — present only in `to`
//   - "removed"  — present only in `from`
//   - "changed"  — present in both, contents differ
//   - "unchanged" — present in both, contents match (filtered out
//     by RenderDiff by default)
type SubmessageDiff struct {
	Group  string // "config" | "module_config" | "channels" | "owner" | "fixed_position"
	Key    string // submessage key, or channel index string
	Status string
	From   json.RawMessage
	To     json.RawMessage
}

// Diff returns the field-level differences between two payloads,
// ordered by group then by canonical key. Order is stable across
// runs so diff output is reproducible.
func Diff(from, to CanonicalPayload) []SubmessageDiff {
	out := []SubmessageDiff{}

	if !jsonEqual(from.Owner, to.Owner) {
		out = append(out, diffPair("owner", "", from.Owner, to.Owner))
	}
	if !jsonEqual(from.FixedPosition, to.FixedPosition) {
		out = append(out, diffPair("fixed_position", "", from.FixedPosition, to.FixedPosition))
	}

	// Channels: align by index. The protocol has fixed slots, so
	// position-based comparison is the right shape.
	n := len(from.Channels)
	if len(to.Channels) > n {
		n = len(to.Channels)
	}
	for i := 0; i < n; i++ {
		var f, t json.RawMessage
		if i < len(from.Channels) {
			f = from.Channels[i]
		}
		if i < len(to.Channels) {
			t = to.Channels[i]
		}
		if !jsonEqual(f, t) {
			out = append(out, diffPair("channels", fmt.Sprintf("%d", i), f, t))
		}
	}

	for _, k := range orderedDiffKeys(from.Config, to.Config, ConfigKeys) {
		f, fOK := from.Config[k]
		t, tOK := to.Config[k]
		if !fOK && !tOK {
			continue
		}
		if !jsonEqual(f, t) {
			out = append(out, diffPair("config", k, f, t))
		}
	}
	for _, k := range orderedDiffKeys(from.ModuleConfig, to.ModuleConfig, ModuleConfigKeys) {
		f, fOK := from.ModuleConfig[k]
		t, tOK := to.ModuleConfig[k]
		if !fOK && !tOK {
			continue
		}
		if !jsonEqual(f, t) {
			out = append(out, diffPair("module_config", k, f, t))
		}
	}
	return out
}

// jsonEqual returns true when two JSON byte slices are semantically
// equal (same logical structure / values), regardless of object key
// order, whitespace, or numeric formatting.
//
// Why this exists: the device-side payload is marshalled in protobuf
// field-number order, but PostgreSQL JSONB normalises object keys
// alphabetically on insert. A naive `bytes.Equal` therefore reports
// every section as "(changed)" the instant a payload round-trips
// through the cloud, even though the values are identical. The diff
// is a developer-facing tool for spotting *real* drift; key-order
// noise drowns out the actual signal.
//
// Empty / nil inputs are treated as equal to each other and to an
// explicit `null`. Invalid JSON on either side falls back to byte
// comparison so we still flag the (extremely unlikely) raw mismatch
// rather than silently swallowing it.
func jsonEqual(a, b json.RawMessage) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		// Treat `null` as equivalent to "no value" — it's how the
		// protobuf-to-JSON path encodes "section absent".
		if isJSONNull(a) && len(b) == 0 {
			return true
		}
		if isJSONNull(b) && len(a) == 0 {
			return true
		}
		return false
	}
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return bytes.Equal(a, b)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(av, bv)
}

func isJSONNull(b json.RawMessage) bool {
	return string(bytes.TrimSpace(b)) == "null"
}

func diffPair(group, key string, f, t json.RawMessage) SubmessageDiff {
	switch {
	case len(f) == 0 && len(t) > 0:
		return SubmessageDiff{Group: group, Key: key, Status: "added", To: t}
	case len(f) > 0 && len(t) == 0:
		return SubmessageDiff{Group: group, Key: key, Status: "removed", From: f}
	default:
		return SubmessageDiff{Group: group, Key: key, Status: "changed", From: f, To: t}
	}
}

func orderedDiffKeys(a, b map[string]json.RawMessage, preferred []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, k := range preferred {
		if _, ok := a[k]; ok {
			out = append(out, k)
			seen[k] = true
			continue
		}
		if _, ok := b[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	trail := []string{}
	for k := range a {
		if !seen[k] {
			trail = append(trail, k)
			seen[k] = true
		}
	}
	for k := range b {
		if !seen[k] {
			trail = append(trail, k)
		}
	}
	sort.Strings(trail)
	return append(out, trail...)
}

// PayloadFromDiff returns a sparse copy of `intended` that contains
// only the sections / channels flagged by `diff`. Used by the apply
// path so we transmit one SetConfig / SetModuleConfig / SetChannel
// admin message per genuinely-changed submessage instead of the
// whole payload on every run.
//
// Why this exists: Meshtastic firmware applies each `Set*` admin
// message as a full submessage write, even when the staged value
// is byte-identical to what's already in flash. Pushing 21
// SetConfig + 8 SetChannel messages on every apply burns radio
// flash, multiplies reboot risk and makes the post-apply drift
// counter ambiguous (which of the 29 messages didn't take?). By
// filtering down to the actual delta first, "drift > 0" tells the
// operator exactly which submessages the firmware refused. See
// the "one reboot per apply" contract — this is the same
// principle extended to the section level.
//
// Channel filtering preserves slot semantics: an absent channel in
// the returned payload means "leave the device's slot alone", not
// "delete it". `Apply` already iterates `Channels` and skips empty
// raw entries, so growing the slice with `nil` placeholders to
// preserve the source index is safe.
func PayloadFromDiff(intended CanonicalPayload, diff []SubmessageDiff) CanonicalPayload {
	out := CanonicalPayload{}
	for _, d := range diff {
		switch d.Group {
		case "owner":
			out.Owner = intended.Owner
		case "fixed_position":
			out.FixedPosition = intended.FixedPosition
		case "channels":
			idx, err := strconv.Atoi(d.Key)
			if err != nil || idx < 0 {
				continue
			}
			for len(out.Channels) <= idx {
				out.Channels = append(out.Channels, nil)
			}
			if idx < len(intended.Channels) {
				out.Channels[idx] = intended.Channels[idx]
			}
		case "config":
			if v, ok := intended.Config[d.Key]; ok {
				if out.Config == nil {
					out.Config = map[string]json.RawMessage{}
				}
				out.Config[d.Key] = v
			}
		case "module_config":
			if v, ok := intended.ModuleConfig[d.Key]; ok {
				if out.ModuleConfig == nil {
					out.ModuleConfig = map[string]json.RawMessage{}
				}
				out.ModuleConfig[d.Key] = v
			}
		}
	}
	return out
}

// ProjectOnto returns a sparse copy of `full` that contains only the
// sections / channel slots that are populated in `shape`. Channel
// slots are matched by index; missing slots in `full` come back as
// empty so the slot count matches `shape` for direction-stable
// diffing.
//
// Why this exists: after an apply-by-diff (`PayloadFromDiff`), the
// post-apply re-read returns the device's *full* state, but the
// `intended` we passed to `Apply` is sparse. Diffing the sparse
// intended against the full actual reports every section the device
// has but we didn't ship as "+ added in actual" — pure noise. The
// drift check should be "for the sections we actually wrote, did the
// device end up where we asked?". Project the actual payload onto
// the intended's shape and diff side-by-side.
func (full CanonicalPayload) ProjectOnto(shape CanonicalPayload) CanonicalPayload {
	out := CanonicalPayload{}
	if len(shape.Owner) > 0 {
		out.Owner = full.Owner
	}
	if len(shape.FixedPosition) > 0 {
		out.FixedPosition = full.FixedPosition
	}
	if len(shape.Channels) > 0 {
		out.Channels = make([]json.RawMessage, len(shape.Channels))
		for i, ch := range shape.Channels {
			if len(ch) == 0 {
				continue
			}
			if i < len(full.Channels) {
				out.Channels[i] = full.Channels[i]
			}
		}
	}
	if len(shape.Config) > 0 {
		out.Config = map[string]json.RawMessage{}
		for k := range shape.Config {
			if v, ok := full.Config[k]; ok {
				out.Config[k] = v
			}
		}
	}
	if len(shape.ModuleConfig) > 0 {
		out.ModuleConfig = map[string]json.RawMessage{}
		for k := range shape.ModuleConfig {
			if v, ok := full.ModuleConfig[k]; ok {
				out.ModuleConfig[k] = v
			}
		}
	}
	return out
}

// CountChanges returns the number of non-unchanged entries.
func CountChanges(d []SubmessageDiff) int {
	n := 0
	for _, x := range d {
		if x.Status != "unchanged" {
			n++
		}
	}
	return n
}

// FieldChange is a single leaf-level delta produced by drilling into a
// section's JSON. A `~ tx_power: 22 → 27` line.
type FieldChange struct {
	Path   string // dotted path within the section, e.g. "settings.name"
	Status string // "added" | "removed" | "changed"
	From   any    // decoded JSON value (only set for "removed" / "changed")
	To     any    // decoded JSON value (only set for "added" / "changed")
}

// FieldChanges expands one SubmessageDiff into its leaf-level changes.
// Used by RenderDiff and exposed for callers that want machine-readable
// drift detail (e.g. agent telemetry).
func FieldChanges(d SubmessageDiff) []FieldChange {
	var out []FieldChange
	walkJSON("", d.From, d.To, &out)
	return out
}

// DiffRenderOptions controls how RenderDiff styles its output.
type DiffRenderOptions struct {
	// FromLabel / ToLabel are echoed in the header so the operator
	// remembers which way the arrow points. Empty values fall back
	// to "from" / "to".
	FromLabel string
	ToLabel   string
	// Color enables lipgloss ANSI styling. Callers should set this
	// from a TTY check on their writer (e.g. golang.org/x/term).
	Color bool
}

// RenderDiff writes a human-readable diff to `w`. Output format:
//
//	device  →  cloud:home/label-demo
//
//	config.lora
//	  ~ tx_power           22 → 27
//	  + use_preset         true
//
//	channels[1].settings
//	  ~ name               "old" → "enterprise"
//
//	3 changes across 2 sections.
//
// On zero changes it prints a single confirmation line so the operator
// knows the device matches the saved snapshot byte-for-byte.
func RenderDiff(w io.Writer, d []SubmessageDiff, opts DiffRenderOptions) {
	st := newDiffStyles(opts.Color)
	from := opts.FromLabel
	if from == "" {
		from = "from"
	}
	to := opts.ToLabel
	if to == "" {
		to = "to"
	}

	if len(d) == 0 {
		fmt.Fprintln(w, st.match.Render("✓ "+from+" matches "+to+" — no differences."))
		return
	}

	fmt.Fprintf(w, "%s %s %s\n\n",
		st.label.Render(from),
		st.arrow.Render("→"),
		st.label.Render(to),
	)

	totalFields := 0
	sectionsWithChanges := 0
	for _, sec := range d {
		fields := FieldChanges(sec)
		if len(fields) == 0 {
			// Whole-section add/remove with empty payload — render as a
			// single line so it doesn't get lost.
			path := sec.Group
			if sec.Key != "" {
				path = sec.Group + "." + sec.Key
			}
			fmt.Fprintln(w, st.section.Render(path))
			fmt.Fprintf(w, "  %s\n\n", st.faint.Render("("+sec.Status+")"))
			sectionsWithChanges++
			totalFields++
			continue
		}
		path := sec.Group
		if sec.Key != "" {
			path = sec.Group + "." + sec.Key
			if sec.Group == "channels" {
				// channels.0 reads more naturally as channels[0].
				path = "channels[" + sec.Key + "]"
			}
		}
		fmt.Fprintln(w, st.section.Render(path))

		// Compute the longest path so values line up in a column.
		maxPath := 0
		for _, f := range fields {
			// `f.Path` may be empty when the diff is on an
			// entire submessage (`module_config.serial: {}` →
			// one FieldChange with Path=""). The renderer
			// substitutes "(value)" for empty paths, so the
			// alignment maximum has to consider the *displayed*
			// path length, not the raw one — otherwise padTo can
			// end up shorter than the rendered token and the
			// rendering pad goes negative (panic).
			disp := f.Path
			if disp == "" {
				disp = "(value)"
			}
			if len(disp) > maxPath {
				maxPath = len(disp)
			}
		}
		for _, f := range fields {
			renderField(w, st, f, maxPath)
		}
		fmt.Fprintln(w)
		sectionsWithChanges++
		totalFields += len(fields)
	}

	noun := "change"
	if totalFields != 1 {
		noun = "changes"
	}
	secNoun := "section"
	if sectionsWithChanges != 1 {
		secNoun = "sections"
	}
	summary := fmt.Sprintf("%d %s across %d %s.", totalFields, noun, sectionsWithChanges, secNoun)
	fmt.Fprintln(w, st.summary.Render(summary))
}

func renderField(w io.Writer, st diffStyles, f FieldChange, padTo int) {
	path := f.Path
	if path == "" {
		path = "(value)"
	}
	// Defensive: if the caller miscomputed padTo (e.g. a future change
	// loops over raw paths instead of displayed ones), clamp to zero
	// rather than panic on a negative repeat count.
	padN := padTo - len(path)
	if padN < 0 {
		padN = 0
	}
	pad := strings.Repeat(" ", padN)
	switch f.Status {
	case "added":
		fmt.Fprintf(w, "  %s %s%s  %s\n",
			st.added.Render("+"),
			st.path.Render(path), pad,
			st.added.Render(formatJSONValue(f.To)),
		)
	case "removed":
		fmt.Fprintf(w, "  %s %s%s  %s\n",
			st.removed.Render("-"),
			st.path.Render(path), pad,
			st.removed.Render(formatJSONValue(f.From)),
		)
	default:
		fmt.Fprintf(w, "  %s %s%s  %s %s %s\n",
			st.changed.Render("~"),
			st.path.Render(path), pad,
			st.removed.Render(formatJSONValue(f.From)),
			st.arrow.Render("→"),
			st.added.Render(formatJSONValue(f.To)),
		)
	}
}

// formatJSONValue renders a decoded JSON value the same way protojson
// would print it, so the operator sees `true` / `"EU_868"` / `5`
// rather than Go's default `%v` quirks.
func formatJSONValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		return t.String()
	case float64:
		// JSON unmarshal hands us float64 for any number; render
		// integers without trailing `.0` and floats compactly.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	}
	// Objects / arrays — fall through to compact JSON. Rare at the
	// leaf because walkJSON drills into them, but possible for
	// heterogeneous shapes.
	out, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	s := string(out)
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}

// walkJSON expands a section-level pair of raw JSON blobs into a flat
// list of leaf-level FieldChange entries. The `prefix` is prepended to
// every emitted path; pass "" at the section root.
func walkJSON(prefix string, from, to json.RawMessage, out *[]FieldChange) {
	fHas := len(from) > 0
	tHas := len(to) > 0
	var fv, tv any
	if fHas {
		_ = json.Unmarshal(from, &fv)
	}
	if tHas {
		_ = json.Unmarshal(to, &tv)
	}
	walkValue(prefix, fHas, fv, tHas, tv, out)
}

func walkValue(path string, fHas bool, fv any, tHas bool, tv any, out *[]FieldChange) {
	switch {
	case !fHas && !tHas:
		return
	case !fHas && tHas:
		emitTree(path, "added", tv, out)
		return
	case fHas && !tHas:
		emitTree(path, "removed", fv, out)
		return
	}

	fm, fIsObj := fv.(map[string]any)
	tm, tIsObj := tv.(map[string]any)
	if fIsObj && tIsObj {
		for _, k := range unionKeys(fm, tm) {
			v1, in1 := fm[k]
			v2, in2 := tm[k]
			walkValue(joinPath(path, k), in1, v1, in2, v2, out)
		}
		return
	}

	fa, fIsArr := fv.([]any)
	ta, tIsArr := tv.([]any)
	if fIsArr && tIsArr {
		n := len(fa)
		if len(ta) > n {
			n = len(ta)
		}
		for i := 0; i < n; i++ {
			var v1, v2 any
			in1 := i < len(fa)
			in2 := i < len(ta)
			if in1 {
				v1 = fa[i]
			}
			if in2 {
				v2 = ta[i]
			}
			walkValue(fmt.Sprintf("%s[%d]", path, i), in1, v1, in2, v2, out)
		}
		return
	}

	if reflect.DeepEqual(fv, tv) {
		return
	}
	*out = append(*out, FieldChange{Path: path, Status: "changed", From: fv, To: tv})
}

// emitTree flattens a one-sided value tree into `+ leaf` / `- leaf`
// entries. Empty objects emit a single placeholder so the section
// header isn't orphaned.
func emitTree(path, status string, v any, out *[]FieldChange) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			*out = append(*out, oneSidedChange(path, status, map[string]any{}))
			return
		}
		for _, k := range sortedMapKeys(t) {
			emitTree(joinPath(path, k), status, t[k], out)
		}
	case []any:
		if len(t) == 0 {
			*out = append(*out, oneSidedChange(path, status, []any{}))
			return
		}
		for i, item := range t {
			emitTree(fmt.Sprintf("%s[%d]", path, i), status, item, out)
		}
	default:
		fc := FieldChange{Path: path, Status: status}
		if status == "added" {
			fc.To = v
		} else {
			fc.From = v
		}
		*out = append(*out, fc)
	}
}

func oneSidedChange(path, status string, v any) FieldChange {
	fc := FieldChange{Path: path, Status: status}
	if status == "added" {
		fc.To = v
	} else {
		fc.From = v
	}
	return fc
}

func unionKeys(a, b map[string]any) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	for k := range b {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	sort.Strings(keys)
	return keys
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// diffStyles bundles the lipgloss styles used by RenderDiff. When
// Color is false every style is a no-op, so the function is safe to
// call from non-TTY callers (CI logs, file redirects).
type diffStyles struct {
	added, removed, changed lipgloss.Style
	section, path           lipgloss.Style
	arrow, faint, label     lipgloss.Style
	match, summary          lipgloss.Style
}

func newDiffStyles(color bool) diffStyles {
	if !color {
		return diffStyles{}
	}
	return diffStyles{
		added:   lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		removed: lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		changed: lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true),
		section: lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true),
		path:    lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		arrow:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		faint:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		label:   lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		match:   lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		summary: lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true),
	}
}
