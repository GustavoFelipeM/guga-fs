package commands

import (
	"fmt"
	"guga-fs/pkg/fs"
)

func HandleMkdir(vfs *fs.VirtualFileSystem, args []string) {
	if len(args) < 1 {
		fmt.Println("mkdir: operando de caminho ausente")
		return
	}
	vfs.Mkdir(args[0])
	vfs.PrintTree(nil, "")
}
