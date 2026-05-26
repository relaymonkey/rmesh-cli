package deviceconfigs

import (
	"errors"
	"fmt"
	"strings"
)

// SourceKind discriminates where a config comes from or goes to in
// the `rmesh device config <verb> --from <src> --to <dst>` grammar
// (D-209).
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

// CloudOwner discriminates a cloud source's owner segment in the
// `cloud:<n>/{mine|template}/<label>` URI grammar (D-213).
type CloudOwner string

const (
	// CloudOwnerEither is the bare-form `cloud:<n>/<label>`. The
	// resolver tries the caller's personal library first and falls
	// back to network templates.
	CloudOwnerEither CloudOwner = ""
	// CloudOwnerMine is the explicit `cloud:<n>/mine/<label>`.
	CloudOwnerMine CloudOwner = "mine"
	// CloudOwnerTemplate is the explicit `cloud:<n>/template/<label>`.
	CloudOwnerTemplate CloudOwner = "template"
)

// Source is a parsed source/destination reference.
type Source struct {
	Kind    SourceKind
	URL     string     // transport URL for SourceDevice
	Path    string     // file path for SourceFile, "-" for SourceStdout
	Network string     // network slug / id / short_id for SourceCloud
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
	// cloud:<network>/<label>
	// cloud:<network>/mine/<label>
	// cloud:<network>/template/<label>
	if strings.HasPrefix(s, "cloud:") {
		body := s[len("cloud:"):]
		if body == "" {
			return Source{}, errors.New("cloud source requires <network>/<label> (e.g. cloud:home/eu-868)")
		}
		// Split on the first `/`: <network>/<rest>
		sep := strings.Index(body, "/")
		if sep < 0 {
			return Source{}, fmt.Errorf("cloud source %q missing /<label> (e.g. cloud:home/eu-868)", raw)
		}
		net := strings.TrimSpace(body[:sep])
		rest := strings.TrimSpace(body[sep+1:])
		if net == "" || rest == "" {
			return Source{}, fmt.Errorf("cloud source %q must be cloud:<network>/<label>", raw)
		}
		owner := CloudOwnerEither
		// Optional owner segment: `mine/...` or `template/...`.
		if sep2 := strings.Index(rest, "/"); sep2 > 0 {
			head := strings.TrimSpace(rest[:sep2])
			tail := strings.TrimSpace(rest[sep2+1:])
			switch head {
			case "mine":
				owner = CloudOwnerMine
				rest = tail
			case "template", "templates", "shared":
				// `template` is canonical; `templates` and `shared`
				// are forgiving aliases (operators tend to remember
				// one or the other and we don't want to scold).
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
			return "cloud:" + s.Network + "/mine/" + s.Label
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
