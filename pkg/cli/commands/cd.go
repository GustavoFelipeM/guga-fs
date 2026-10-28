package commands

import "guga-fs/pkg/fs"

func HandleCd(vfs *fs.VirtualFileSystem, args []string) {
	path := "/"
	if len(args) > 0 {
		path = args[0]
	}
	vfs.Cd(path)
}
