package commands

import (
	"fmt"
	"guga-fs/pkg/fs"
)

func HandleCat(vfs *fs.VirtualFileSystem, args []string) {
	if len(args) < 1 {
		fmt.Println("cat: operando ausente")
		return
	}
	vfs.Read(args[0])
}
