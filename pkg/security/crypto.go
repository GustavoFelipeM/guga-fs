package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const kdfIterations = 20000

func DeriveKey(passphrase, salt string) []byte {
	data := []byte(salt + passphrase)
	sum := sha256.Sum256(data)
	for i := 1; i < kdfIterations; i++ {
		buf := make([]byte, 0, len(sum)+len(data))
		buf = append(buf, sum[:]...)
		buf = append(buf, data...)
		sum = sha256.Sum256(buf)
	}
	return sum[:]
}

func KeyCheck(key []byte) string {
	buf := append(append([]byte(nil), key...), []byte("guga-keycheck")...)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

func VerifyKey(key []byte, check string) bool {
	return SecureEqual(KeyCheck(key), check)
}

func ivFor(salt string, idx int) []byte {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|bloco|%d", salt, idx)))
	return sum[:aes.BlockSize]
}

func CryptBlocks(key []byte, salt string, data []byte, blockSize int) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	idx := 0
	for off := 0; off < len(data); off += blockSize {
		end := off + blockSize
		if end > len(data) {
			end = len(data)
		}
		stream := cipher.NewCTR(block, ivFor(salt, idx))
		stream.XORKeyStream(out[off:end], data[off:end])
		idx++
	}
	return out, nil
}

func HexPreview(b []byte, n int) string {
	if len(b) == 0 {
		return "0x(vazio)"
	}
	if n > len(b) {
		n = len(b)
	}
	return "0x" + strings.ToUpper(hex.EncodeToString(b[:n])) + "..."
}
