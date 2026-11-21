package security

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
)

func BlockHash(data []byte, blockSize int) string {
	buf := make([]byte, blockSize)
	copy(buf, data)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

func (s *System) RecordBlock(block int, data []byte, blockSize int) {
	if s == nil {
		return
	}
	s.Hashes[block] = BlockHash(data, blockSize)
}

func (s *System) ForgetBlock(block int) {
	if s == nil {
		return
	}
	delete(s.Hashes, block)
}

func (s *System) Expected(block int) (string, bool) {
	h, ok := s.Hashes[block]
	return h, ok
}

type Signature struct {
	Name    string
	Pattern string
}

var Signatures = []Signature{
	{Name: "EICAR-Test-File", Pattern: "EICAR-STANDARD-ANTIVIRUS-TEST-FILE"},
	{Name: "Trojan.Guga.Dropper", Pattern: "MALWARE_SIGNATURE_X"},
	{Name: "Worm.Shell.Wipe", Pattern: "rm -rf /"},
	{Name: "Exploit.SQLi.Drop", Pattern: "DROP TABLE"},
}

func ScanSignatures(data []byte) (Signature, bool) {
	for _, sig := range Signatures {
		if bytes.Contains(data, []byte(sig.Pattern)) {
			return sig, true
		}
	}
	return Signature{}, false
}
