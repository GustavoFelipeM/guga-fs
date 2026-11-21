package fs

import "guga-fs/pkg/security"

type Inode struct {
	ID           int
	IsDir        bool
	Name         string
	SizeBytes    int
	InodeBlock   int
	DirectBlocks []int
	Parent       *Inode
	Children     map[string]*Inode
	Content      string
	Mode         uint32

	// Etapa 3: segurança
	Owner     string
	Group     string
	ACL       []security.ACLEntry
	Encrypted bool
	CryptSalt string
	KeyCheck  string
}

type BlockInfo struct {
	Tag         string
	Description string
}

type IOBackend interface {
	WriteBlock(block int, data []byte, label string)
	ReadBlock(block int, label string) []byte
	FreeBlock(block int)
	Flush(operation string)
}


type RawIO interface {
	PeekBlock(block int) ([]byte, error)
	PokeBlock(block int, data []byte) error
	RawWriteBlock(block int, data []byte) error
	RepairBlock(block int) ([]byte, error)
	RedundancyName() string
}
