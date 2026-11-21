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
	"read_file":      HandleCat,
	"login":          HandleLogin,
	"logout":         HandleLogout,
	"su":             HandleSu,
	"whoami":         HandleWhoami,
	"useradd":        HandleUseradd,
	"groupadd":       HandleGroupadd,
	"usermod":        HandleUsermod,
	"passwd":         HandlePasswd,
	"users":          HandleUsers,
	"chown":          HandleChown,
	"setfacl":        HandleSetfacl,
	"getfacl":        HandleGetfacl,
	"encrypt_file":   HandleEncryptFile,
	"decrypt_file":   HandleDecryptFile,
	"antivirus_scan": HandleAntivirusScan,
	"disk_tamper":    HandleDiskTamper,
	"audit_log":      HandleAuditLog,
}
