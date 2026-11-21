package commands

import (
	"fmt"
	"strings"

	"guga-fs/pkg/fs"
	"guga-fs/pkg/security"
)

func HandleChown(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) || !vfs.Sec.RequireKernel("chown") {
		return
	}
	if len(args) < 2 {
		fmt.Println("Uso: chown <usuario>[:<grupo>] <caminho>")
		return
	}
	owner, group := args[0], ""
	if idx := strings.Index(args[0], ":"); idx >= 0 {
		owner, group = args[0][:idx], args[0][idx+1:]
	}
	vfs.Chown(args[1], owner, group)
}

func HandleSetfacl(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) {
		return
	}
	const usage = "Uso: setfacl -m <u|g>:<nome>:<rwx> <caminho> | -x <u|g>:<nome> <caminho> | -b <caminho>"
	if len(args) < 2 {
		fmt.Println(usage)
		return
	}
	switch args[0] {
	case "-m":
		if len(args) < 3 {
			fmt.Println(usage)
			return
		}
		entry, err := security.ParseACLSpec(args[1], true)
		if err != nil {
			fmt.Printf("setfacl: %v\n", err)
			return
		}
		vfs.SetFacl(args[2], entry, false, false)
	case "-x":
		if len(args) < 3 {
			fmt.Println(usage)
			return
		}
		entry, err := security.ParseACLSpec(args[1], false)
		if err != nil {
			fmt.Printf("setfacl: %v\n", err)
			return
		}
		vfs.SetFacl(args[2], entry, true, false)
	case "-b":
		vfs.SetFacl(args[1], security.ACLEntry{}, false, true)
	default:
		fmt.Println(usage)
	}
}

func HandleGetfacl(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) {
		return
	}
	if len(args) < 1 {
		fmt.Println("Uso: getfacl <caminho>")
		return
	}
	vfs.GetFacl(args[0])
}
