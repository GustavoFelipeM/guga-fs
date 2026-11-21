package commands

import (
	"fmt"
	"strings"

	"guga-fs/pkg/fs"
)

func HandleEncryptFile(vfs *fs.VirtualFileSystem, args []string) {
	rest, key := splitKey(args)
	if len(rest) < 1 || key == "" {
		fmt.Println("Uso: encrypt_file <caminho> --key \"<chave>\"")
		return
	}
	vfs.EncryptFile(rest[0], key)
}

func HandleDecryptFile(vfs *fs.VirtualFileSystem, args []string) {
	rest, key := splitKey(args)
	if len(rest) < 1 || key == "" {
		fmt.Println("Uso: decrypt_file <caminho> --key \"<chave>\"")
		return
	}
	vfs.DecryptFile(rest[0], key)
}

func HandleAntivirusScan(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) {
		return
	}
	path, repair := "/", false
	for _, a := range args {
		if a == "--repair" {
			repair = true
		} else {
			path = a
		}
	}

	if repair && !vfs.Sec.RequireKernel("antivirus_scan --repair") {
		return
	}
	vfs.AntivirusScan(path, repair)
}

func HandleDiskTamper(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) || !vfs.Sec.RequireKernel("disk_tamper") {
		return
	}
	if len(args) < 1 {
		fmt.Println("Uso: disk_tamper <arquivo> [\"dados\"]")
		return
	}
	vfs.TamperBlock(args[0], strings.Trim(strings.Join(args[1:], " "), "\"'"))
}

func HandleAuditLog(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) || !vfs.Sec.RequireKernel("audit_log") {
		return
	}
	n := 20
	if len(args) > 0 {
		if _, err := fmt.Sscanf(args[0], "%d", &n); err != nil || n < 1 {
			n = 20
		}
	}
	log := vfs.Sec.Audit
	start := 0
	if len(log) > n {
		start = len(log) - n
	}
	fmt.Printf("\n[LOG DE AUDITORIA - últimos %d de %d eventos]\n", len(log)-start, len(log))
	for _, e := range log[start:] {
		status := "PERMITIDO"
		if !e.Allowed {
			status = "NEGADO   "
		}
		fmt.Printf("%s | %-9s | %-8s | %-22s | %s\n", e.Time.Format("15:04:05"), status, e.User, e.Event, e.Detail)
	}
}
