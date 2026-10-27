package commands

import "guga-fs/pkg/fs"

func HandleFsStatus(vfs *fs.VirtualFileSystem, args []string) {
	details := false
	for _, arg := range args {
		if arg == "--details" || arg == "-d" {
			details = true
		}
	}
	vfs.FsStatus(details)
}
