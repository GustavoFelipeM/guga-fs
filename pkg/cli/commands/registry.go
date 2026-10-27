package commands

import "guga-fs/pkg/fs"

// Assinatura padrão para qualquer comando
type CommandHandler func(vfs *fs.VirtualFileSystem, args []string)

// Mapeamento global de comandos
var Registry = map[string]CommandHandler{
	"mkdir":     HandleMkdir,
	"create":    HandleCreate,
	"fs_status": HandleFsStatus,
	"chmod":     HandleChmod,
}
