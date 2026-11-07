package commands

import "guga-fs/pkg/fs"

type CommandHandler func(vfs *fs.VirtualFileSystem, args []string)

var Registry = map[string]CommandHandler{
	"cd":         HandleCd,
	"ls":         HandleLs,
	"mkdir":      HandleMkdir,
	"create":     HandleCreate,
	"write_file": HandleWriteFile,
	"iosched":    HandleIOSched,
	"raid":       HandleRaid,
	"cat":        HandleCat,
	"rm":         HandleRm,
	"tree":       HandleTree,
	"fs_status":  HandleFsStatus,
	"chmod":      HandleChmod,
	"clear":      HandleClear,
}
