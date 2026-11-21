package commands

import (
	"fmt"
	"guga-fs/pkg/fs"
)

func HandleCat(vfs *fs.VirtualFileSystem, args []string) {
	rest, key := splitKey(args)
	if len(rest) < 1 {
		fmt.Println("cat: operando ausente")
		return
	}
	vfs.ReadWithKey(rest[0], key)
}
