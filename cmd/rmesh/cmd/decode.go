package cmd

import (
	"bufio"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/relaymonkey/relaymesh-edge/internal/decode"
	"github.com/relaymonkey/relaymesh-edge/internal/decrypt"
)

var (
	decodePSK   string
	decodeStrip bool
)

var decodeCmd = &cobra.Command{
	Use:   "decode",
	Short: "Enrich observe JSONL with decoded payloads (stdin → stdout)",
	Long: `Read observe JSONL from stdin and write JSONL with a cloud-shaped
decoded field when mesh_packet_b64 is present.

  rmesh agent observe | rmesh decode
  rmesh agent observe | rmesh decode --psk Ag==
  rmesh agent observe | rmesh decode --strip | jq 'select(.portnum == 71)'

Default --psk is AQ== (Meshtastic LongFast default channel key).`,
	RunE: runDecode,
}

func init() {
	rootCmd.AddCommand(decodeCmd)
	decodeCmd.Flags().StringVar(&decodePSK, "psk", decrypt.DefaultPSKInput,
		"Channel PSK for decrypting encrypted packets (Meshtastic export form, e.g. AQ==)")
	decodeCmd.Flags().BoolVar(&decodeStrip, "strip", false,
		"Drop mesh_packet_b64 from output after decode (smaller lines for jq)")
	decodeCmd.SilenceUsage = true
}

func runDecode(cmd *cobra.Command, args []string) error {
	if _, err := decrypt.NormaliseMeshtasticPSK(decodePSK); err != nil {
		return fmt.Errorf("--psk: %w", err)
	}
	return streamDecode(cmd.InOrStdin(), cmd.OutOrStdout(), decodePSK, decodeStrip)
}

func streamDecode(in io.Reader, out io.Writer, psk string, stripWire bool) error {
	sc := bufio.NewScanner(in)
	// Observe lines include base64 MeshPackets; default 64 KiB is too small.
	buf := make([]byte, 0, 256*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		enriched, err := decode.EnrichObserveLine(line, psk, stripWire)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, string(enriched)); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}
