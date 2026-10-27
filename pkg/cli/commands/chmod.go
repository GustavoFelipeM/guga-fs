package commands

import (
	"fmt"
	"guga-fs/pkg/fs"
	"strconv"
)

func HandleChmod(vfs *fs.VirtualFileSystem, args []string) {
	if len(args) < 2 {
		fmt.Println("Uso: chmod <modo_octal> <caminho> (ex: chmod 755 /sistema)")
		return
	}

	mode, err := strconv.ParseUint(args[0], 8, 32)
	if err != nil {
		fmt.Println("chmod: modo octal inválido")
		return
	}

	target := vfs.ResolvePath(args[1])
	if target == nil {
		fmt.Printf("chmod: '%s' não encontrado\n", args[1])
		return
	}

	target.Mode = uint32(mode)
	fmt.Printf("Permissões de '%s' alteradas para %o\n", args[1], mode)
}
