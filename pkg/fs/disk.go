package fs

import (
	"strconv"
	"strings"
)

type VirtualFileSystem struct {
	BlockSize   int
	TotalBlocks int
	Blocks      []*BlockInfo
	Inodes      map[int]*Inode
	NextInodeID int
	Root        *Inode
	CurrentDir  *Inode
	IO          IOBackend
}

func NewVFS(blockSize, totalBlocks int) *VirtualFileSystem {
	vfs := &VirtualFileSystem{
		BlockSize:   blockSize,
		TotalBlocks: totalBlocks,
		Blocks:      make([]*BlockInfo, totalBlocks),
		Inodes:      make(map[int]*Inode),
		NextInodeID: 0,
	}

	vfs.Root = vfs.CreateInode(true, "")
	vfs.Root.InodeBlock = 0
	vfs.Root.DirectBlocks = append(vfs.Root.DirectBlocks, 0)
	vfs.Blocks[0] = &BlockInfo{
		Tag:         "INODE_RAIZ",
		Description: "Diretório '/'",
	}
	vfs.CurrentDir = vfs.Root
	return vfs
}

func (v *VirtualFileSystem) CreateInode(isDir bool, name string) *Inode {
	inode := &Inode{
		ID:         v.NextInodeID,
		IsDir:      isDir,
		Name:       name,
		Children:   make(map[string]*Inode),
		SizeBytes:  0,
		Mode:       0755, // permissão padrão
		InodeBlock: -1,
	}
	v.Inodes[v.NextInodeID] = inode
	v.NextInodeID++
	return inode
}

func (v *VirtualFileSystem) GetFreeBlocks() []int {
	free := []int{}
	for i, block := range v.Blocks {
		if block == nil {
			free = append(free, i)
		}
	}
	return free
}

func (v *VirtualFileSystem) ResolvePath(path string) *Inode {
	if path == "" || path == "/" {
		return v.Root
	}

	parts := []string{}
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}

	curr := v.CurrentDir
	if strings.HasPrefix(path, "/") {
		curr = v.Root
	}

	for _, part := range parts {
		if part == "." {
			continue
		} else if part == ".." {
			if curr.Parent != nil {
				curr = curr.Parent
			}
		} else {
			child, exists := curr.Children[part]
			if exists {
				curr = child
			} else {
				return nil
			}
		}
	}
	return curr
}

func (v *VirtualFileSystem) GetFullPath(inode *Inode) string {
	if inode == nil {
		inode = v.CurrentDir
	}
	if inode == v.Root {
		return "/"
	}
	pathParts := []string{}
	curr := inode
	for curr != nil && curr.Parent != nil {
		pathParts = append([]string{curr.Name}, pathParts...)
		curr = curr.Parent
	}
	return "/" + strings.Join(pathParts, "/")
}

func ParseSize(sizeStr string) int {
	s := strings.ToUpper(strings.TrimSpace(sizeStr))
	if strings.HasSuffix(s, "MB") {
		val, _ := strconv.ParseFloat(strings.TrimSuffix(s, "MB"), 64)
		return int(val * 1024 * 1024)
	} else if strings.HasSuffix(s, "KB") {
		val, _ := strconv.ParseFloat(strings.TrimSuffix(s, "KB"), 64)
		return int(val * 1024)
	} else if strings.HasSuffix(s, "B") {
		val, _ := strconv.Atoi(strings.TrimSuffix(s, "B"))
		return val
	}
	val, err := strconv.Atoi(s)
	if err == nil {
		return val
	}
	return 1024
}
