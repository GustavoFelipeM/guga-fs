package commands

import (
	"fmt"
	"guga-fs/pkg/fs"
)

func HandleCreate(vfs *fs.VirtualFileSystem, args []string) {
	if len(args) < 1 {
		fmt.Println("Uso: create <caminho> [--size <tamanho>]")
		return
	}

	path := args[0]
	sizeStr := "1KB"

	for i, arg := range args {
		if arg == "--size" && i+1 < len(args) {
			sizeStr = args[i+1]
			break
		}
	}

	vfs.Create(path, sizeStr)
	vfs.PrintTree(nil, "")
}
