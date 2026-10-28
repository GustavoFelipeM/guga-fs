package commands

import "guga-fs/pkg/fs"

func HandleTree(vfs *fs.VirtualFileSystem, args []string) {
	vfs.PrintTree(nil, "")
}
