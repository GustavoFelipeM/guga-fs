package commands

import "guga-fs/pkg/fs"

// Assinatura padrão para qualquer comando
type CommandHandler func(vfs *fs.VirtualFileSystem, args []string)

// Mapeamento global de comandos
var Registry = map[string]CommandHandler{
	"cd":        HandleCd,
	"ls":        HandleLs,
	"mkdir":     HandleMkdir,
	"create":    HandleCreate,
	"cat":       HandleCat,
	"rm":        HandleRm,
	"tree":      HandleTree,
	"fs_status": HandleFsStatus,
	"chmod":     HandleChmod,
	"clear":     HandleClear,
}
