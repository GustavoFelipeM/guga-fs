package main

import (
	"fmt"
	"os"

	"guga-fs/pkg/cli"
	"guga-fs/pkg/fs"
	"guga-fs/pkg/security"
	"guga-fs/pkg/storage"
)

func main() {
	const (
		blockSize   = 4096
		totalBlocks = 16
		tracks      = 200
	)

	vfs := fs.NewVFS(blockSize, totalBlocks)

	io, err := storage.NewController(blockSize, totalBlocks, tracks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao iniciar a camada de E/S:", err)
		os.Exit(1)
	}
	vfs.IO = io
	vfs.Sec = security.NewSystem() // Etapa 3: usuários, grupos, ACL, auditoria e integridade

	cli.RunShell(vfs)
}
