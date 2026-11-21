package main

import (
	"fmt"
	"os"

	"guga-fs/pkg/cli"
	"guga-fs/pkg/fs"
	"guga-fs/pkg/memory"
	"guga-fs/pkg/security"
	"guga-fs/pkg/storage"
)

func main() {
	const (
		blockSize   = 4096
		totalBlocks = 16
		tracks      = 200

		// Etapa 4: o swap ocupa as últimas 50 trilhas do disco (150..199).
		swapBaseTrack = 150
	)

	vfs := fs.NewVFS(blockSize, totalBlocks)

	io, err := storage.NewController(blockSize, totalBlocks, tracks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao iniciar a camada de E/S:", err)
		os.Exit(1)
	}
	vfs.IO = io
	vfs.Sec = security.NewSystem() // Etapa 3: usuários, grupos, ACL, auditoria e integridade


	// Etapa 4: memória híbrida (área reservada contígua + RAM dinâmica paginada).
	// Cada leitura/escrita de swap entra na fila do escalonador de disco da Etapa 2
	// e é atendida junto com as requisições do sistema de arquivos no próximo Flush.
	vfs.Mem = memory.NewManager()
	vfs.Mem.SwapIO = func(write bool, slot int) int {
		track := swapBaseTrack
		if slot >= 0 && vfs.Mem.SwapSlots > 0 {
			track = swapBaseTrack + slot*(tracks-swapBaseTrack)/vfs.Mem.SwapSlots
		}
		io.Background = append(io.Background, track)
		return track
	}

	cli.RunShell(vfs)
}
