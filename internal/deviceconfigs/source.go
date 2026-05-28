package deviceconfigs

import (
	"errors"
	"fmt"
	"strings"
)

// SourceKind discriminates where a config comes from or goes to in
// the `rmesh device config <verb> --from <src> --to <dst>` grammar.
type SourceKind int

const (
	// SourceUnknown is the zero value.
	SourceUnknown SourceKind = iota
	// SourceDevice is a live Meshtastic device; the URL field holds
	// the transport URL ("serial:/dev/...", "http://...", or "" to
	// mean "use the agent config's transport.url").
	SourceDevice
	// SourceFile is a local JSON / YAML file; Path holds the path.
	SourceFile
	// SourceCloud is a saved cloud config; Network + Label resolve
	// the row. Label may be a uuid or a human label.
	SourceCloud
	// SourceStdout / SourceStdin only show up as `--to -` / future
	// `--from -`. Path is "-".
	SourceStdout
)

// CloudOwner discriminates a cloud source's owner segment. Per
// D-219 personal rows are user-scoped, so the grammar is:
//
//	cloud:mine/<label>             — owner=mine, no network
//	cloud:<n>/template/<label>     — owner=template, on network n
//	cloud:<n>/<label>              — owner=template (bare form), on network n
type CloudOwner string

const (
	// CloudOwnerEither is the bare-form `cloud:<n>/<label>`.
	// Per D-219 the resolver treats it as a template lookup (the
	// pre-D-219 mine-first-then-template fallback is gone — personal
	// is no longer per-network).
	CloudOwnerEither CloudOwner = ""
	// CloudOwnerMine is the explicit `cloud:mine/<label>`. Carries
	// no Network — personal rows are user-scoped per D-219.
	CloudOwnerMine CloudOwner = "mine"
	// CloudOwnerTemplate is the explicit `cloud:<n>/template/<label>`.
	CloudOwnerTemplate CloudOwner = "template"
)

// Source is a parsed source/destination reference.
type Source struct {
	Kind SourceKind
	URL  string // transport URL for SourceDevice
	Path string // file path for SourceFile, "-" for SourceStdout
	// Network is the network slug / id / short_id for cloud refs
	// that target a network (templates). Empty for `cloud:mine/...`.
	Network string
	Owner   CloudOwner // mine / template / either, for SourceCloud
	Label   string     // config label or id for SourceCloud
	Raw     string     // verbatim user input, kept for error messages
}

// ParseSource resolves a `--from` / `--to` token. The grammar:
//
//	device                       → SourceDevice (uses agent config URL)
//	device:<transport-url>       → SourceDevice with explicit URL
//	file:<path>                  → SourceFile
//	<path-with-/-or-.>           → SourceFile (file: prefix optional)
//	cloud:<network>/<label-or-id> → SourceCloud
//	-                            → SourceStdout (only valid as --to)
//
// Unknown shapes return an error with a readable suggestion.
func ParseSource(raw string) (Source, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Source{}, errors.New("empty source")
	}
	if s == "-" {
		return Source{Kind: SourceStdout, Path: "-", Raw: raw}, nil
	}
	// device | device:<url>
	if s == "device" {
		return Source{Kind: SourceDevice, Raw: raw}, nil
	}
	if strings.HasPrefix(s, "device:") {
		return Source{Kind: SourceDevice, URL: s[len("device:"):], Raw: raw}, nil
	}
	// file:<path>
	if strings.HasPrefix(s, "file:") {
		return Source{Kind: SourceFile, Path: s[len("file:"):], Raw: raw}, nil
	}
	// cloud:mine/<label>             (D-219, no network)
	// cloud:<network>/<label>        (bare → template-only on <network>)
	// cloud:<network>/template/<label>
	// cloud:<network>/mine/<label>   (legacy, accepted but normalised
	//                                  to cloud:mine/<label>; the
	//                                  network segment is dropped)
	if strings.HasPrefix(s, "cloud:") {
		body := s[len("cloud:"):]
		if body == "" {
			return Source{}, errors.New("cloud source requires mine/<label> or <network>/<label> (e.g. cloud:mine/eu-868 or cloud:home/eu-868)")
		}
		// `cloud:mine/<label>` — user-scoped personal library.
		// No network segment; if one follows, it's an error.
		if strings.HasPrefix(body, "mine/") {
			label := strings.TrimSpace(strings.TrimPrefix(body, "mine/"))
			if label == "" {
				return Source{}, fmt.Errorf("cloud source %q must end with a label (e.g. cloud:mine/eu-868)", raw)
			}
			return Source{
				Kind:  SourceCloud,
				Owner: CloudOwnerMine,
				Label: label,
				Raw:   raw,
			}, nil
		}
		sep := strings.Index(body, "/")
		if sep < 0 {
			return Source{}, fmt.Errorf("cloud source %q missing /<label> (e.g. cloud:home/eu-868)", raw)
		}
		net := strings.TrimSpace(body[:sep])
		rest := strings.TrimSpace(body[sep+1:])
		if net == "" || rest == "" {
			return Source{}, fmt.Errorf("cloud source %q must be cloud:mine/<label> or cloud:<network>/<label>", raw)
		}
		owner := CloudOwnerEither
		// Optional owner segment: `mine/...` (legacy) or `template/...`.
		if sep2 := strings.Index(rest, "/"); sep2 > 0 {
			head := strings.TrimSpace(rest[:sep2])
			tail := strings.TrimSpace(rest[sep2+1:])
			switch head {
			case "mine":
				// Legacy `cloud:<n>/mine/<label>` — drop the network
				// segment per D-219 and normalise to mine/<label>.
				return Source{
					Kind:  SourceCloud,
					Owner: CloudOwnerMine,
					Label: tail,
					Raw:   raw,
				}, nil
			case "template", "templates", "shared":
				owner = CloudOwnerTemplate
				rest = tail
			}
		}
		if rest == "" {
			return Source{}, fmt.Errorf("cloud source %q must end with a label", raw)
		}
		return Source{
			Kind:    SourceCloud,
			Network: net,
			Owner:   owner,
			Label:   rest,
			Raw:     raw,
		}, nil
	}
	// Implicit file: token contains a path separator or a known
	// extension; this lets `--from ./config.yaml` work without
	// requiring the `file:` prefix.
	if looksLikePath(s) {
		return Source{Kind: SourceFile, Path: s, Raw: raw}, nil
	}
	return Source{}, fmt.Errorf("could not parse %q (expected device, device:<url>, file:<path>, cloud:<network>/<label>, or - )", raw)
}

// looksLikePath returns true when the token is unambiguously a file
// path: contains `/`, starts with `.`, or ends with `.yaml`/`.yml`/`.json`.
func looksLikePath(s string) bool {
	if strings.ContainsRune(s, '/') {
		return true
	}
	if strings.HasPrefix(s, ".") {
		return true
	}
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		if strings.HasSuffix(strings.ToLower(s), ext) {
			return true
		}
	}
	return false
}

// String renders a Source back to its canonical form, useful for
// echo-back diagnostics.
func (s Source) String() string {
	switch s.Kind {
	case SourceDevice:
		if s.URL != "" {
			return "device:" + s.URL
		}
		return "device"
	case SourceFile:
		return "file:" + s.Path
	case SourceCloud:
		switch s.Owner {
		case CloudOwnerMine:
			return "cloud:mine/" + s.Label
		case CloudOwnerTemplate:
			return "cloud:" + s.Network + "/template/" + s.Label
		default:
			return "cloud:" + s.Network + "/" + s.Label
		}
	case SourceStdout:
		return "-"
	}
	if s.Raw != "" {
		return s.Raw
	}
	return "<unknown>"
}
