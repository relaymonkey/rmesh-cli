package decrypt

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// DefaultPSKInput is the Meshtastic export for the well-known LongFast key.
const DefaultPSKInput = "AQ=="

// MeshtasticDefaultPSK is the firmware LongFast 16-byte key (short PSK 0x01).
var MeshtasticDefaultPSK = []byte{
	0xd4, 0xf1, 0xbb, 0x3a, 0x20, 0x29, 0x07, 0x59,
	0xf0, 0xbc, 0xff, 0xab, 0xcf, 0x4e, 0x69, 0x01,
}

// NormaliseMeshtasticPSK accepts Meshtastic channel PSK forms (AQ==, hex, base64).
func NormaliseMeshtasticPSK(in string) ([]byte, error) {
	in = strings.TrimSpace(in)

	hexCandidate := strings.TrimPrefix(strings.ToLower(in), "0x")
	if isHex(hexCandidate) && len(hexCandidate)%2 == 0 && len(hexCandidate) > 0 {
		if decoded, err := hex.DecodeString(hexCandidate); err == nil {
			if k, ok := keyFromBytes(decoded); ok {
				return k, nil
			}
		}
	}

	for _, dec := range []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	} {
		if decoded, err := dec(in); err == nil {
			if k, ok := keyFromBytes(decoded); ok {
				return k, nil
			}
		}
	}

	return nil, errors.New("psk must be a 1-byte short PSK (e.g. AQ==), or 16/32 bytes as hex or base64")
}

func keyFromBytes(b []byte) ([]byte, bool) {
	switch len(b) {
	case 1:
		return expandShortPSK(b[0])
	case 16, 32:
		out := make([]byte, len(b))
		copy(out, b)
		return out, true
	default:
		return nil, false
	}
}

func expandShortPSK(b byte) ([]byte, bool) {
	if b == 0x00 || b > 0x0A {
		return nil, false
	}
	out := make([]byte, len(MeshtasticDefaultPSK))
	copy(out, MeshtasticDefaultPSK)
	if b == 0x01 {
		return out, true
	}
	out[15] = MeshtasticDefaultPSK[15] + (b - 1)
	return out, true
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// Context carries nonce inputs for Meshtastic AES-CTR decrypt.
type Context struct {
	PacketID     uint64
	SourceNodeID uint32
}

// DecryptAESCTRPSK decrypts a MeshPacket.encrypted blob (meshtastic-aes-ctr-psk).
func DecryptAESCTRPSK(blob, key []byte, ctx Context) ([]byte, error) {
	if len(key) != 16 && len(key) != 32 {
		return nil, fmt.Errorf("decrypt: key must be 16 or 32 bytes (got %d)", len(key))
	}
	block, err := aesNewCipher(key)
	if err != nil {
		return nil, err
	}
	return aesCTRDecrypt(block, blob, ctx)
}
