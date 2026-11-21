//go:build js && wasm

package main

import (
	"fmt"
	"io"
	"syscall/js"

	"guga-fs/pkg/cli"
	"guga-fs/pkg/fs"
	"guga-fs/pkg/memory"
	"guga-fs/pkg/process"
	"guga-fs/pkg/security"
	"guga-fs/pkg/storage"
)

func main() {
	const (
		blockSize     = 4096
		totalBlocks   = 16
		tracks        = 200
		swapBaseTrack = 150
	)

	vfs := fs.NewVFS(blockSize, totalBlocks)
	ioCtrl, err := storage.NewController(blockSize, totalBlocks, tracks)
	if err != nil {
		fmt.Println("erro ao iniciar E/S:", err)
		return
	}
	vfs.IO = ioCtrl
	vfs.Sec = security.NewSystem()

	vfs.Mem = memory.NewManager()
	vfs.Mem.SwapIO = func(write bool, slot int) int {
		track := swapBaseTrack
		if slot >= 0 && vfs.Mem.SwapSlots > 0 {
			track = swapBaseTrack + slot*(tracks-swapBaseTrack)/vfs.Mem.SwapSlots
		}
		ioCtrl.Background = append(ioCtrl.Background, track)
		return track
	}

	vfs.Proc = process.NewManager(vfs.Mem, ioCtrl)

	pr, pw := io.Pipe()
	js.Global().Set("goSendInput", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			_, _ = pw.Write([]byte(args[0].String()))
		}
		return nil
	}))

	cli.RunShellWithInput(vfs, pr)
	select {}
}
