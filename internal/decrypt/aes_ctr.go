package decrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

func aesNewCipher(key []byte) (cipher.Block, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("decrypt: aes new cipher: %w", err)
	}
	return block, nil
}

func aesCTRDecrypt(block cipher.Block, blob []byte, ctx Context) ([]byte, error) {
	var iv [16]byte
	binary.LittleEndian.PutUint64(iv[0:8], ctx.PacketID)
	binary.LittleEndian.PutUint32(iv[8:12], ctx.SourceNodeID)
	stream := cipher.NewCTR(block, iv[:])
	out := make([]byte, len(blob))
	stream.XORKeyStream(out, blob)
	return out, nil
}
