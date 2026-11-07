package fs

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
