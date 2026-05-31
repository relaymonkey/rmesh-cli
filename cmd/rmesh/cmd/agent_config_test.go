package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestParseLabelOverrides(t *testing.T) {
	labels, err := parseLabelOverrides([]string{"site=east", "role=observer"})
	if err != nil {
		t.Fatal(err)
	}
	if labels["site"] != "east" || labels["role"] != "observer" {
		t.Fatalf("labels = %#v", labels)
	}

	if _, err := parseLabelOverrides([]string{"bad"}); err == nil {
		t.Fatal("expected error for malformed label")
	}
}

// TestVerbFlagSurfaces locks the per-verb flag contract from D-220. If a verb
// regains a flag it doesn't read (or loses one it does), this test fails — and
// the regression is visible in PR review.
func TestVerbFlagSurfaces(t *testing.T) {
	cases := []struct {
		cmd      *cobra.Command
		mustHave []string
		mustNot  []string
	}{
		{
			cmd: runCmd,
			mustHave: []string{
				"agent-id", "transport-url", "ble-pin",
				"mqtt-broker-url", "mqtt-username", "ignore-ok-to-mqtt",
				"synthesise-position-interval", "label", "set",
			},
		},
		{
			cmd: observeCmd,
			mustHave: []string{
				"agent-id", "transport-url", "ignore-ok-to-mqtt",
				"synthesise-position-interval", "label", "set",
			},
			// observe never publishes — MQTT flags would be a lie.
			mustNot: []string{
				"mqtt-broker-url", "mqtt-username", "mqtt-password",
				"mqtt-topic-prefix", "mqtt-client-id",
			},
		},
		{
			cmd: doctorCmd,
			mustHave: []string{
				"agent-id", "transport-url",
				"mqtt-broker-url", "synthesise-position-interval",
				"label", "set",
			},
			// doctor validates connectivity; it never forwards packets, so the
			// consent filter is not its concern.
			mustNot: []string{"ignore-ok-to-mqtt"},
		},
		{
			cmd: pairCmd,
			// pair talks to the cloud API, not the radio.
			mustNot: []string{
				"agent-id", "transport-url", "ble-pin",
				"mqtt-broker-url", "synthesise-position-interval",
				"ignore-ok-to-mqtt", "label", "set",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.cmd.Name(), func(t *testing.T) {
			for _, name := range tc.mustHave {
				if tc.cmd.Flags().Lookup(name) == nil {
					t.Errorf("expected flag --%s on %s", name, tc.cmd.Name())
				}
			}
			for _, name := range tc.mustNot {
				if tc.cmd.Flags().Lookup(name) != nil {
					t.Errorf("unexpected flag --%s on %s", name, tc.cmd.Name())
				}
			}
		})
	}
}
