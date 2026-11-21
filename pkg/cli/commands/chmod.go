package commands

import (
	"fmt"
	"strconv"

	"guga-fs/pkg/fs"
)

func HandleChmod(vfs *fs.VirtualFileSystem, args []string) {
	if len(args) < 2 {
		fmt.Println("Uso: chmod <modo_octal> <caminho> (ex: chmod 755 /sistema)")
		return
	}

	mode, err := strconv.ParseUint(args[0], 8, 32)
	if err != nil || mode > 0777 {
		fmt.Println("chmod: modo octal inválido (use 000..777)")
		return
	}

	vfs.Chmod(args[1], uint32(mode))
}
