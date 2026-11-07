package commands

import (
	"fmt"

	"guga-fs/pkg/fs"
	"guga-fs/pkg/storage"
)

func getController(vfs *fs.VirtualFileSystem) *storage.Controller {
	c, ok := vfs.IO.(*storage.Controller)
	if !ok || c == nil {
		fmt.Println("E/S: camada de disco não inicializada.")
		return nil
	}
	return c
}
