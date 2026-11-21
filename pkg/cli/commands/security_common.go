package commands

import (
	"fmt"
	"strings"

	"guga-fs/pkg/fs"
)

func splitKey(args []string) (rest []string, key string) {
	for i := 0; i < len(args); i++ {
		if args[i] != "--key" {
			rest = append(rest, args[i])
			continue
		}
		parts := []string{}
		for i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			parts = append(parts, args[i+1])
			i++
		}
		key = strings.Trim(strings.Join(parts, " "), "\"'")
	}
	return rest, key
}

func requireLogin(vfs *fs.VirtualFileSystem) bool {
	if vfs.Sec == nil || vfs.Sec.Current == nil {
		fmt.Println("Nenhum usuário autenticado. Use: login --user <nome>")
		return false
	}
	return true
}

func readNewPassword(vfs *fs.VirtualFileSystem) (string, bool) {
	if vfs.Sec.ReadSecret == nil {
		fmt.Println("Entrada de senha indisponível.")
		return "", false
	}
	p1 := vfs.Sec.ReadSecret("Nova senha: ")
	p2 := vfs.Sec.ReadSecret("Confirme a senha: ")
	if p1 != p2 {
		fmt.Println("As senhas não conferem.")
		return "", false
	}
	return p1, true
}
