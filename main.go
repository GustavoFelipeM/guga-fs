package main

import (
	"guga-fs/pkg/cli"
	"guga-fs/pkg/fs"
)

func main() {
	// Inicializa o sistema de arquivos (Blocos de 4KB, 16 blocos total)
	vfs := fs.NewVFS(4096, 16)

	// Executa o Shell interativo
	cli.RunShell(vfs)
}
