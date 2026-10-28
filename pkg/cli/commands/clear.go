package commands

import (
	"fmt"
	"guga-fs/pkg/fs"
)

func HandleClear(vfs *fs.VirtualFileSystem, args []string) {
	fmt.Print("\033[H\033[2J")
}
