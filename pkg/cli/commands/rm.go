package commands

import (
	"fmt"
	"guga-fs/pkg/fs"
)

func HandleRm(vfs *fs.VirtualFileSystem, args []string) {
	if len(args) < 1 {
		fmt.Println("rm: operando ausente")
		return
	}
	vfs.Delete(args[0])
}
