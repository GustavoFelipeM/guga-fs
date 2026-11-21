package commands

import (
	"fmt"
	"strings"

	"guga-fs/pkg/fs"
	"guga-fs/pkg/security"
)

func startSession(vfs *fs.VirtualFileSystem, u *security.User) {
	vfs.Sec.Login(u)
	vfs.CurrentDir = vfs.Root
	if home := vfs.ResolvePath("/home/" + u.Name); home != nil && home.IsDir {
		vfs.CurrentDir = home
	}
	vfs.Sec.Record("LOGIN", true, u.Name)
	fmt.Printf("Autenticado com sucesso. Usuário: '%s' | Grupo: '%s' | Modo: %s\n", u.Name, u.Group, vfs.Sec.Mode)
}

func authenticate(vfs *fs.VirtualFileSystem, name string) {
	if vfs.Sec.ReadSecret == nil {
		fmt.Println("Entrada de senha indisponível.")
		return
	}
	pass := vfs.Sec.ReadSecret("Senha: ")
	u, ok := vfs.Sec.Authenticate(name, pass)
	if !ok {
		fmt.Println("Falha na autenticação: usuário ou senha inválidos.")
		vfs.Sec.Record("LOGIN_FALHOU", false, name)
		return
	}
	startSession(vfs, u)
}

func HandleLogin(vfs *fs.VirtualFileSystem, args []string) {
	name := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--user" && i+1 < len(args) {
			name = args[i+1]
			i++
		} else if !strings.HasPrefix(args[i], "--") && name == "" {
			name = args[i]
		}
	}
	if name == "" {
		fmt.Println("Uso: login --user <nome>")
		return
	}
	authenticate(vfs, name)
}

func HandleLogout(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) {
		return
	}
	vfs.Sec.Record("LOGOUT", true, vfs.Sec.CurrentName())
	fmt.Printf("Sessão de '%s' encerrada.\n", vfs.Sec.CurrentName())
	vfs.Sec.Logout()
	vfs.CurrentDir = vfs.Root
}

func HandleSu(vfs *fs.VirtualFileSystem, args []string) {
	target := security.RootUser
	if len(args) > 0 {
		target = args[0]
	}
	authenticate(vfs, target)
}

func HandleWhoami(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) {
		return
	}
	u := vfs.Sec.Current
	groups := u.Group
	if len(u.Groups) > 0 {
		groups += "," + strings.Join(u.Groups, ",")
	}
	fmt.Printf("Usuário: %s (uid=%d) | Grupos: %s | Modo: %s\n", u.Name, u.UID, groups, vfs.Sec.Mode)
}

func HandleSudo(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) {
		return
	}
	if len(args) == 0 {
		fmt.Println("Uso: sudo <comando> [argumentos]")
		return
	}
	name := strings.ToLower(args[0])
	switch name {
	case "sudo", "login", "su", "logout":
		fmt.Printf("sudo: o comando '%s' não pode ser executado com sudo.\n", name)
		return
	}
	handler, ok := Registry[name]
	if !ok {
		fmt.Printf("sudo: comando '%s' não encontrado.\n", name)
		return
	}

	sec := vfs.Sec
	if !sec.IsKernel() {
		if !sec.CanSudo(sec.Current) {
			fmt.Printf("sudo: '%s' não pertence ao grupo '%s' e não pode elevar privilégios.\n", sec.CurrentName(), security.AdminGroup)
			fmt.Println("Tentativa não autorizada registrada nos logs do sistema.")
			sec.Record("SUDO_NEGADO", false, strings.Join(args, " "))
			return
		}
		if sec.ReadSecret == nil {
			fmt.Println("Entrada de senha indisponível.")
			return
		}
		pass := sec.ReadSecret(fmt.Sprintf("[sudo] senha de '%s': ", sec.CurrentName()))
		if _, ok := sec.Authenticate(sec.CurrentName(), pass); !ok {
			fmt.Println("sudo: senha incorreta.")
			sec.Record("SUDO_SENHA_INCORRETA", false, strings.Join(args, " "))
			return
		}
	}
	sec.Record("SUDO", true, strings.Join(args, " "))
	sec.WithKernel(func() { handler(vfs, args[1:]) })
}

func init() {
	Registry["sudo"] = HandleSudo
}

// ---------- Usuários e grupos ----------

func HandleUseradd(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) || !vfs.Sec.RequireKernel("useradd") {
		return
	}
	if len(args) < 1 {
		fmt.Println("Uso: useradd <nome> [--group <grupo>]")
		return
	}
	name, group := args[0], "users"
	for i := 1; i < len(args); i++ {
		if args[i] == "--group" && i+1 < len(args) {
			group = args[i+1]
			i++
		}
	}
	if _, exists := vfs.Sec.Users[name]; exists {
		fmt.Printf("useradd: o usuário '%s' já existe\n", name)
		return
	}
	if _, ok := vfs.Sec.Groups[group]; !ok {
		fmt.Printf("useradd: o grupo '%s' não existe (use groupadd)\n", group)
		return
	}
	pass, ok := readNewPassword(vfs)
	if !ok {
		return
	}
	u, err := vfs.Sec.AddUser(name, group, pass)
	if err != nil {
		fmt.Printf("useradd: %v\n", err)
		return
	}
	vfs.EnsureHome(u.Name, u.Group)
	vfs.Sec.Record("USERADD", true, fmt.Sprintf("%s (grupo %s)", u.Name, u.Group))
	fmt.Printf("Usuário '%s' (uid=%d) criado no grupo '%s'. Senha armazenada como SHA-256 com salt. Home: /home/%s\n", u.Name, u.UID, u.Group, u.Name)
}

func HandleGroupadd(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) || !vfs.Sec.RequireKernel("groupadd") {
		return
	}
	if len(args) < 1 {
		fmt.Println("Uso: groupadd <grupo>")
		return
	}
	if err := vfs.Sec.AddGroup(args[0]); err != nil {
		fmt.Printf("groupadd: %v\n", err)
		return
	}
	vfs.Sec.Record("GROUPADD", true, args[0])
	fmt.Printf("Grupo '%s' criado (gid=%d).\n", args[0], vfs.Sec.Groups[args[0]].GID)
}

func HandleUsermod(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) || !vfs.Sec.RequireKernel("usermod") {
		return
	}
	if len(args) < 3 || (args[0] != "-aG" && args[0] != "--add-group") {
		fmt.Println("Uso: usermod -aG <grupo> <usuario>")
		return
	}
	if err := vfs.Sec.AddToGroup(args[2], args[1]); err != nil {
		fmt.Printf("usermod: %v\n", err)
		return
	}
	vfs.Sec.Record("USERMOD", true, fmt.Sprintf("%s += %s", args[2], args[1]))
	fmt.Printf("Usuário '%s' adicionado ao grupo '%s'.\n", args[2], args[1])
}

func HandlePasswd(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) {
		return
	}
	sec := vfs.Sec
	target := sec.Current
	if len(args) > 0 {
		u, ok := sec.Users[args[0]]
		if !ok {
			fmt.Printf("passwd: usuário '%s' não existe\n", args[0])
			return
		}
		if u != sec.Current && !sec.RequireKernel("passwd "+args[0]) {
			return
		}
		target = u
	}
	if !sec.IsKernel() {
		old := sec.ReadSecret("Senha atual: ")
		if _, ok := sec.Authenticate(target.Name, old); !ok {
			fmt.Println("passwd: senha atual incorreta.")
			sec.Record("PASSWD_FALHOU", false, target.Name)
			return
		}
	}
	pass, ok := readNewPassword(vfs)
	if !ok {
		return
	}
	if err := sec.SetPassword(target, pass); err != nil {
		fmt.Printf("passwd: %v\n", err)
		return
	}
	sec.Record("PASSWD", true, target.Name)
	fmt.Printf("Senha de '%s' atualizada (novo salt gerado).\n", target.Name)
}

func HandleUsers(vfs *fs.VirtualFileSystem, args []string) {
	if !requireLogin(vfs) {
		return
	}
	showHash := false
	for _, a := range args {
		if a == "--show-hash" {
			showHash = true
		}
	}
	if showHash && !vfs.Sec.RequireKernel("users --show-hash") {
		return
	}
	fmt.Println("\n[USUÁRIOS]")
	for _, u := range vfs.Sec.SortedUsers() {
		extra := ""
		if len(u.Groups) > 0 {
			extra = " (+ " + strings.Join(u.Groups, ", ") + ")"
		}
		fmt.Printf("  uid=%-5d %-10s grupo=%s%s\n", u.UID, u.Name, u.Group, extra)
		if showHash {
			fmt.Printf("            salt=%s\n            sha256(salt+senha)=%s\n", security.ShortHash(u.Salt), u.Hash)
		}
	}
	fmt.Println("[GRUPOS]")
	for _, g := range vfs.Sec.SortedGroups() {
		members := []string{}
		for _, u := range vfs.Sec.SortedUsers() {
			if vfs.Sec.IsMember(u, g.Name) {
				members = append(members, u.Name)
			}
		}
		fmt.Printf("  gid=%-5d %-10s membros: %s\n", g.GID, g.Name, strings.Join(members, ", "))
	}
}
