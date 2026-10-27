package fs

type Inode struct {
	ID           int
	IsDir        bool
	Name         string
	SizeBytes    int
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
