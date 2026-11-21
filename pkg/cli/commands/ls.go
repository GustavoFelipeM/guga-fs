package commands

import "guga-fs/pkg/fs"

func HandleLs(vfs *fs.VirtualFileSystem, args []string) {
	path := "."
	long := false
	for _, a := range args {
		if a == "-l" {
			long = true
		} else {
			path = a
		}
	}
	if long {
		vfs.ListLong(path)
		return
	}
	vfs.Read(path)
}
