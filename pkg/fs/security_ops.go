package fs

import (
	"fmt"
	"sort"
	"strings"

	"guga-fs/pkg/security"
)

// ---------- Controle de acesso (rwx + ACL) ----------
func (v *VirtualFileSystem) CheckAccess(node *Inode, want security.Perm) bool {
	if v.Sec == nil || node == nil {
		return true
	}
	if v.Sec.Allowed(node.Owner, node.Group, node.Mode, node.ACL, want) {
		return true
	}

	user := v.Sec.CurrentName()
	path := v.GetFullPath(node)
	what := security.PermName(want)

	fmt.Println("\n[SISTEMA DE SEGURANÇA - VERIFICAÇÃO DE ACL]")
	fmt.Printf("Objeto      : %s\n", path)
	fmt.Printf("Proprietário: %s (Grupo: %s)\n", node.Owner, node.Group)
	fmt.Printf("Permissões  : %s (%03o)\n", security.ModeString(node.Mode), node.Mode&0777)
	if len(node.ACL) > 0 {
		fmt.Printf("ACL estendida: %s\n", security.FormatACL(node.ACL))
	}
	fmt.Printf("Ação Pedida : %s por '%s' (Grupo: %s)\n\n", strings.ToUpper(what), user, v.Sec.CurrentGroup())
	fmt.Printf("[NEGADO]: O usuário '%s' não possui permissão de %s no objeto '%s'.\n", user, what, path)
	fmt.Println("Tentativa não autorizada registrada nos logs do sistema.")
	v.Sec.Record("ACL_NEGADO", false, fmt.Sprintf("%s em %s", what, path))
	return false
}

func (v *VirtualFileSystem) requireOwner(node *Inode, op string) bool {
	if v.Sec == nil || v.Sec.IsKernel() {
		return true
	}
	if v.Sec.Current != nil && v.Sec.Current.Name == node.Owner {
		return true
	}
	fmt.Printf("%s: operação não permitida: apenas o proprietário ('%s') ou o Modo Kernel pode alterar '%s'.\n",
		op, node.Owner, v.GetFullPath(node))
	fmt.Println("Tentativa não autorizada registrada nos logs do sistema.")
	v.Sec.Record("DONO_REQUERIDO", false, fmt.Sprintf("%s em %s", op, v.GetFullPath(node)))
	return false
}

func (v *VirtualFileSystem) Chmod(path string, mode uint32) {
	target := v.ResolvePath(path)
	if target == nil {
		fmt.Printf("chmod: '%s' não encontrado\n", path)
		return
	}
	if !v.requireOwner(target, "chmod") {
		return
	}
	target.Mode = mode & 0777
	v.persistInode(target)
	v.Sec.Record("CHMOD", true, fmt.Sprintf("%s -> %03o", v.GetFullPath(target), target.Mode))
	fmt.Printf("Permissões de '%s' alteradas para %03o (%s)\n", path, target.Mode, security.ModeString(target.Mode))
}

func (v *VirtualFileSystem) Chown(path, owner, group string) {
	target := v.ResolvePath(path)
	if target == nil {
		fmt.Printf("chown: '%s' não encontrado\n", path)
		return
	}
	if owner != "" {
		if _, ok := v.Sec.Users[owner]; !ok {
			fmt.Printf("chown: usuário '%s' não existe\n", owner)
			return
		}
	}
	if group != "" {
		if _, ok := v.Sec.Groups[group]; !ok {
			fmt.Printf("chown: grupo '%s' não existe\n", group)
			return
		}
	}
	if owner != "" {
		target.Owner = owner
	}
	if group != "" {
		target.Group = group
	}
	v.persistInode(target)
	v.Sec.Record("CHOWN", true, fmt.Sprintf("%s -> %s:%s", v.GetFullPath(target), target.Owner, target.Group))
	fmt.Printf("Proprietário de '%s' alterado para %s:%s\n", path, target.Owner, target.Group)
}

func (v *VirtualFileSystem) SetFacl(path string, entry security.ACLEntry, remove, clear bool) {
	target := v.ResolvePath(path)
	if target == nil {
		fmt.Printf("setfacl: '%s' não encontrado\n", path)
		return
	}
	if !v.requireOwner(target, "setfacl") {
		return
	}
	switch {
	case clear:
		target.ACL = nil
		fmt.Printf("ACL estendida de '%s' removida.\n", path)
	case remove:
		var ok bool
		target.ACL, ok = security.RemoveACL(target.ACL, entry.Kind, entry.Name)
		if !ok {
			fmt.Printf("setfacl: entrada '%s:%s' não existe em '%s'\n", entry.Kind, entry.Name, path)
			return
		}
		fmt.Printf("Entrada '%s:%s' removida da ACL de '%s'.\n", entry.Kind, entry.Name, path)
	default:
		if entry.Kind == "u" {
			if _, ok := v.Sec.Users[entry.Name]; !ok {
				fmt.Printf("setfacl: usuário '%s' não existe\n", entry.Name)
				return
			}
		} else if _, ok := v.Sec.Groups[entry.Name]; !ok {
			fmt.Printf("setfacl: grupo '%s' não existe\n", entry.Name)
			return
		}
		target.ACL = security.SetACL(target.ACL, entry)
		fmt.Printf("ACL de '%s' atualizada: %s\n", path, entry)
	}
	v.persistInode(target)
	v.Sec.Record("SETFACL", true, v.GetFullPath(target))
}

func (v *VirtualFileSystem) GetFacl(path string) {
	n := v.ResolvePath(path)
	if n == nil {
		fmt.Printf("getfacl: '%s' não encontrado\n", path)
		return
	}
	fmt.Printf("# file: %s\n# owner: %s\n# group: %s\n", v.GetFullPath(n), n.Owner, n.Group)
	fmt.Printf("user::%s\n", security.Perm((n.Mode>>6)&7))
	for _, e := range n.ACL {
		if e.Kind == "u" {
			fmt.Printf("user:%s:%s\n", e.Name, e.Perms)
		}
	}
	fmt.Printf("group::%s\n", security.Perm((n.Mode>>3)&7))
	for _, e := range n.ACL {
		if e.Kind == "g" {
			fmt.Printf("group:%s:%s\n", e.Name, e.Perms)
		}
	}
	fmt.Printf("other::%s\n", security.Perm(n.Mode&7))
}

func (v *VirtualFileSystem) ListLong(path string) {
	n := v.ResolvePath(path)
	if n == nil {
		fmt.Printf("ls: '%s' não encontrado.\n", path)
		return
	}
	if !v.CheckAccess(n, security.PermR) {
		return
	}
	line := func(x *Inode) {
		kind := "-"
		name := x.Name
		if x.IsDir {
			kind = "d"
			name += "/"
		}
		acl := " "
		if len(x.ACL) > 0 {
			acl = "+"
		}
		extra := ""
		if x.Encrypted {
			extra = " [CRIPTO]"
		}
		fmt.Printf("%s%s%s %-8s %-8s %6d  %s%s\n", kind, security.ModeString(x.Mode), acl, x.Owner, x.Group, x.SizeBytes, name, extra)
	}
	if !n.IsDir {
		line(n)
		return
	}
	names := make([]string, 0, len(n.Children))
	for name := range n.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		line(n.Children[name])
	}
}

// ---------- Metadados persistidos nos blocos ----------

func (v *VirtualFileSystem) inodeMeta(node *Inode) []byte {
	return []byte(fmt.Sprintf("INODE id=%d caminho=%s tamanho=%d modo=%o dono=%s:%s acl=%d cripto=%t blocos=%v",
		node.ID, v.GetFullPath(node), node.SizeBytes, node.Mode, node.Owner, node.Group, len(node.ACL), node.Encrypted, node.DirectBlocks))
}

func (v *VirtualFileSystem) persistInode(node *Inode) {
	if v.IO == nil || node.IsDir || node.InodeBlock < 0 {
		return
	}
	raw, ok := v.IO.(RawIO)
	if !ok {
		return
	}
	meta := v.inodeMeta(node)
	if err := raw.PokeBlock(node.InodeBlock, meta); err != nil {
		fmt.Printf("[E/S] ERRO ao atualizar o i-node: %v\n", err)
		return
	}
	v.Sec.RecordBlock(node.InodeBlock, meta, v.BlockSize)
}

func (v *VirtualFileSystem) peekBlock(block int) []byte {
	if raw, ok := v.IO.(RawIO); ok {
		if data, err := raw.PeekBlock(block); err == nil {
			return data
		}
		return make([]byte, v.BlockSize)
	}
	return v.IO.ReadBlock(block, "varredura")
}

func (v *VirtualFileSystem) EnsureHome(user, group string) {
	home, ok := v.Root.Children["home"]
	if !ok {
		home = v.CreateInode(true, "home")
		home.Parent = v.Root
		home.Owner, home.Group = security.RootUser, security.AdminGroup
		v.Root.Children["home"] = home
	}
	if _, exists := home.Children[user]; exists {
		return
	}
	dir := v.CreateInode(true, user)
	dir.Parent = home
	dir.Owner, dir.Group = user, group
	dir.Mode = 0755
	home.Children[user] = dir
}

// ---------- Criptografia em repouso ----------

func (v *VirtualFileSystem) decryptBuffer(node *Inode, key string, buf []byte) ([]byte, error) {
	k := security.DeriveKey(key, node.CryptSalt)
	if !security.VerifyKey(k, node.KeyCheck) {
		return nil, fmt.Errorf("chave incorreta")
	}
	return security.CryptBlocks(k, node.CryptSalt, buf, v.BlockSize)
}

func (v *VirtualFileSystem) fileBytes(node *Inode) []byte {
	buf := []byte{}
	for _, b := range node.DirectBlocks {
		buf = append(buf, v.peekBlock(b)...)
	}
	if len(buf) > node.SizeBytes {
		buf = buf[:node.SizeBytes]
	}
	return buf
}

func (v *VirtualFileSystem) writeBack(node *Inode, data []byte, operation string) []byte {
	full := v.GetFullPath(node)
	meta := v.inodeMeta(node)
	v.IO.WriteBlock(node.InodeBlock, meta, fmt.Sprintf("INODE '%s'", full))
	v.Sec.RecordBlock(node.InodeBlock, meta, v.BlockSize)
	for idx, b := range node.DirectBlocks {
		start := idx * v.BlockSize
		end := start + v.BlockSize
		if end > len(data) {
			end = len(data)
		}
		part := data[start:end]
		v.IO.WriteBlock(b, part, fmt.Sprintf("'%s' - Parte %d/%d", full, idx+1, len(node.DirectBlocks)))
		v.Sec.RecordBlock(b, part, v.BlockSize)
	}
	v.IO.Flush(fmt.Sprintf("%s %s", operation, full))
	return data
}

func (v *VirtualFileSystem) EncryptFile(path, key string) {
	node := v.ResolvePath(path)
	if node == nil {
		fmt.Printf("encrypt_file: '%s' não encontrado.\n", path)
		return
	}
	if node.IsDir {
		fmt.Printf("encrypt_file: '%s' é um diretório.\n", path)
		return
	}
	if v.IO == nil {
		fmt.Println("encrypt_file: camada de E/S não inicializada.")
		return
	}
	if key == "" {
		fmt.Println("Uso: encrypt_file <caminho> --key \"<chave>\"")
		return
	}
	if node.Encrypted {
		fmt.Printf("encrypt_file: '%s' já está criptografado.\n", path)
		return
	}
	if !v.CheckAccess(node, security.PermR) || !v.CheckAccess(node, security.PermW) {
		return
	}

	plain := v.fileBytes(node)
	salt := security.NewSalt()
	k := security.DeriveKey(key, salt)
	cipherText, err := security.CryptBlocks(k, salt, plain, v.BlockSize)
	if err != nil {
		fmt.Printf("encrypt_file: %v\n", err)
		return
	}

	node.Encrypted = true
	node.CryptSalt = salt
	node.KeyCheck = security.KeyCheck(k)
	node.Content = "" 

	v.writeBack(node, cipherText, "encrypt_file")
	v.Sec.Record("CRIPTOGRAFAR", true, v.GetFullPath(node))

	fmt.Printf("Arquivo '%s' criptografado com sucesso.\n", v.GetFullPath(node))
	fmt.Printf("Conteúdo gravado nos blocos físicos: \"%s (Encrypted Ciphertext)\"\n", security.HexPreview(cipherText, 8))
}

func (v *VirtualFileSystem) DecryptFile(path, key string) {
	node := v.ResolvePath(path)
	if node == nil {
		fmt.Printf("decrypt_file: '%s' não encontrado.\n", path)
		return
	}
	if node.IsDir || !node.Encrypted {
		fmt.Printf("decrypt_file: '%s' não está criptografado.\n", path)
		return
	}
	if v.IO == nil {
		fmt.Println("decrypt_file: camada de E/S não inicializada.")
		return
	}
	if !v.CheckAccess(node, security.PermR) || !v.CheckAccess(node, security.PermW) {
		return
	}

	plain, err := v.decryptBuffer(node, key, v.fileBytes(node))
	if err != nil {
		fmt.Printf("decrypt_file: %v\n", err)
		v.Sec.Record("DECIFRAR_FALHOU", false, v.GetFullPath(node))
		return
	}
	node.Encrypted = false
	node.CryptSalt = ""
	node.KeyCheck = ""

	v.writeBack(node, plain, "decrypt_file")
	v.Sec.Record("DECIFRAR", true, v.GetFullPath(node))
	fmt.Printf("Arquivo '%s' decifrado com sucesso (conteúdo em texto claro nos blocos).\n", v.GetFullPath(node))
}

// ---------- Integridade: antivírus simulado ----------

func (v *VirtualFileSystem) collectFiles(node *Inode, out *[]*Inode) {
	if !node.IsDir {
		*out = append(*out, node)
		return
	}
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		v.collectFiles(node.Children[name], out)
	}
}

func fileBlocks(n *Inode) []int {
	blocks := []int{}
	if n.InodeBlock >= 0 {
		blocks = append(blocks, n.InodeBlock)
	}
	return append(blocks, n.DirectBlocks...)
}


func (v *VirtualFileSystem) AntivirusScan(path string, repair bool) {
	root := v.ResolvePath(path)
	if root == nil {
		fmt.Printf("antivirus_scan: '%s' não encontrado.\n", path)
		return
	}
	if v.IO == nil {
		fmt.Println("antivirus_scan: camada de E/S não inicializada.")
		return
	}
	raw, hasRaw := v.IO.(RawIO)

	files := []*Inode{}
	v.collectFiles(root, &files)

	fmt.Println("\n[MÓDULO DE SEGURANÇA - VARREDURA DE INTEGRIDADE]")
	fmt.Printf("Analisando %d arquivos em busca de assinaturas maliciosas e hashes corrompidos...\n", len(files))

	clean, alerts, infected, repaired := 0, 0, 0, 0
	for _, f := range files {
		full := v.GetFullPath(f)

		bad := []int{}
		for _, b := range fileBlocks(f) {
			expected, ok := v.Sec.Expected(b)
			actual := security.BlockHash(v.peekBlock(b), v.BlockSize)
			if !ok || expected != actual {
				bad = append(bad, b)
			}
		}

		if len(bad) > 0 {
			alerts++
			fmt.Printf("- %-26s : [ALERTA] Hash do bloco difere do registro oficial!\n", full)
			fmt.Printf("  Bloco(s) afetado(s): %v\n", bad)
			fmt.Println("  Resultado: Possível modificação direta não autorizada no bloco do disco.")
			v.Sec.Record("ANTIVIRUS_ALERTA", false, fmt.Sprintf("%s blocos %v", full, bad))

			if repair && hasRaw {
				for _, b := range bad {
					rec, err := raw.RepairBlock(b)
					expected, ok := v.Sec.Expected(b)
					if err != nil || !ok || security.BlockHash(rec, v.BlockSize) != expected {
						fmt.Printf("  Reparo do bloco %02d: [FALHA] redundância (%s) não restaurou o conteúdo oficial.\n", b, raw.RedundancyName())
						continue
					}
					if err := raw.RawWriteBlock(b, rec); err != nil {
						fmt.Printf("  Reparo do bloco %02d: [FALHA] %v\n", b, err)
						continue
					}
					repaired++
					fmt.Printf("  Reparo do bloco %02d: restaurado via redundância (%s) [OK]\n", b, raw.RedundancyName())
					v.Sec.Record("ANTIVIRUS_REPARO", true, fmt.Sprintf("%s bloco %d", full, b))
				}
			}
			continue
		}

		if !f.Encrypted {
			if sig, found := security.ScanSignatures(v.fileBytes(f)); found {
				infected++
				fmt.Printf("- %-26s : [INFECTADO] Assinatura '%s' detectada!\n", full, sig.Name)
				v.Sec.Record("ANTIVIRUS_INFECTADO", false, fmt.Sprintf("%s (%s)", full, sig.Name))
				if repair {
					f.Mode, f.Owner, f.Group = 0, security.RootUser, security.AdminGroup
					f.ACL = nil
					v.persistInode(f)
					fmt.Println("  Quarentena aplicada: dono root:admin, permissões 000.")
				}
				continue
			}
		}

		clean++
		if f.Encrypted {
			fmt.Printf("- %-26s : [OK - Criptografado e Íntegro]\n", full)
		} else {
			fmt.Printf("- %-26s : [OK - Íntegro]\n", full)
		}
	}

	fmt.Printf("\nResumo: %d analisados | %d íntegros | %d com hash divergente | %d infectados", len(files), clean, alerts, infected)
	if repair {
		fmt.Printf(" | %d bloco(s) reparado(s)", repaired)
	}
	fmt.Println()
	v.Sec.Record("ANTIVIRUS_SCAN", true, fmt.Sprintf("%s: %d arquivos, %d alertas, %d infectados", path, len(files), alerts, infected))
}


func (v *VirtualFileSystem) TamperBlock(path string, data string) {
	node := v.ResolvePath(path)
	if node == nil || node.IsDir {
		fmt.Printf("disk_tamper: '%s' não é um arquivo válido.\n", path)
		return
	}
	raw, ok := v.IO.(RawIO)
	if !ok || len(node.DirectBlocks) == 0 {
		fmt.Println("disk_tamper: arquivo sem blocos de dados ou E/S indisponível.")
		return
	}
	b := node.DirectBlocks[0]
	if data == "" {
		data = "!!! DADOS ADULTERADOS !!!"
	}
	if err := raw.RawWriteBlock(b, []byte(data)); err != nil {
		fmt.Printf("disk_tamper: %v\n", err)
		return
	}
	v.Sec.Record("DISK_TAMPER", true, fmt.Sprintf("%s bloco %d", v.GetFullPath(node), b))
	fmt.Printf("[DISCO] Escrita direta no bloco físico %02d de '%s' (FS, ACL e paridade ignorados).\n", b, v.GetFullPath(node))
}
