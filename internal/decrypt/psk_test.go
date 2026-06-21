package decrypt

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestDecryptKnownEnvelope(t *testing.T) {
	encrypted, err := hex.DecodeString("d1febe296f05a105167914b799c2291118d156d29160721491fdc503cc290295abdc")
	if err != nil {
		t.Fatalf("decode encrypted: %v", err)
	}
	got, err := DecryptAESCTRPSK(encrypted, MeshtasticDefaultPSK, Context{
		PacketID:     0x3ceb474f,
		SourceNodeID: 0x9ee784a0,
	})
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	want, _ := hex.DecodeString("0803121c0d0080b5231500807506188d01254a2a046a28017800800100b801104801")
	if !bytes.Equal(got, want) {
		t.Fatalf("plaintext mismatch:\n got %x\nwant %x", got, want)
	}
}

func TestNormaliseMeshtasticPSK(t *testing.T) {
	key, err := NormaliseMeshtasticPSK(DefaultPSKInput)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 16 || !bytes.Equal(key, MeshtasticDefaultPSK) {
		t.Fatalf("AQ== key mismatch: %x", key)
	}
	key, err = NormaliseMeshtasticPSK(base64.StdEncoding.EncodeToString(MeshtasticDefaultPSK))
	if err != nil || len(key) != 16 {
		t.Fatalf("raw16 base64: %v len=%d", err, len(key))
	}
	if _, err := NormaliseMeshtasticPSK("not-a-key"); err == nil {
		t.Fatal("expected error for garbage psk")
	}
}
