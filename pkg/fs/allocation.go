package fs

import (
	"fmt"
	"sort"
	"strings"

	"guga-fs/pkg/security"
)

func splitPath(path string) (parentPath, name string, ok bool) {
	parts := []string{}
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "", "", false
	}
	name = parts[len(parts)-1]
	if name == "." || name == ".." {
		return "", "", false
	}
	dir := strings.Join(parts[:len(parts)-1], "/")
	if strings.HasPrefix(path, "/") {
		return "/" + dir, name, true
	}
	if dir == "" {
		return ".", name, true
	}
	return dir, name, true
}

func (v *VirtualFileSystem) Mkdir(path string) {
	parentPath, dirName, ok := splitPath(path)
	if !ok {
		fmt.Println("mkdir: caminho inválido")
		return
	}

	parent := v.ResolvePath(parentPath)
	if parent == nil || !parent.IsDir {
		fmt.Printf("mkdir: diretório pai não encontrado para '%s'\n", path)
		return
	}
	if _, exists := parent.Children[dirName]; exists {
		fmt.Printf("mkdir: não foi possível criar '%s': Já existe\n", dirName)
		return
	}

	if !v.CheckAccess(parent, security.PermW) {
		return
	}

	newDir := v.CreateInode(true, dirName)
	newDir.Parent = parent
	parent.Children[dirName] = newDir
}

func (v *VirtualFileSystem) Create(path, sizeStr string) {
	v.writeFile("create", path, "", ParseSize(sizeStr), false)
}

func (v *VirtualFileSystem) WriteFile(path, content, sizeStr string) {
	size := 0
	if sizeStr != "" {
		size = ParseSize(sizeStr)
	}
	if size == 0 && content == "" {
		size = 1024
	}
	v.writeFile("write_file", path, content, size, true)
}

func buildPayload(content string, size int, name string) []byte {
	buf := make([]byte, size)
	n := copy(buf, content)
	if content == "" && n < size {
		pattern := []byte(name + "|")
		for i := 0; i < size; i++ {
			buf[i] = pattern[i%len(pattern)]
		}
	}
	return buf
}

func (v *VirtualFileSystem) writeFile(cmd, path, content string, size int, overwrite bool) {
	parentPath, name, ok := splitPath(path)
	if !ok {
		fmt.Printf("%s: caminho inválido\n", cmd)
		return
	}

	parent := v.ResolvePath(parentPath)
	if parent == nil || !parent.IsDir {
		fmt.Printf("%s: diretório pai não encontrado.\n", cmd)
		return
	}

	existing, exists := parent.Children[name]
	if exists && existing.IsDir {
		fmt.Printf("%s: '%s' é um diretório.\n", cmd, name)
		return
	}
	if exists && !overwrite {
		fmt.Printf("%s: arquivo '%s' já existe.\n", cmd, name)
		return
	}
	if exists {
		if !v.CheckAccess(existing, security.PermW) {
			return
		}
	} else if !v.CheckAccess(parent, security.PermW) {
		return
	}

	if size < len(content) {
		size = len(content)
	}
	neededData := (size + v.BlockSize - 1) / v.BlockSize

	required := neededData
	available := len(v.GetFreeBlocks())
	if exists {
		available += len(existing.DirectBlocks)
	} else {
		required++
	}
	if available < required {
		fmt.Printf("Erro: Espaço insuficiente (%d blocos necessários, %d livres).\n", required, available)
		return
	}

	if exists {
		for _, b := range existing.DirectBlocks {
			v.Blocks[b] = nil
			v.Sec.ForgetBlock(b)
			if v.IO != nil {
				v.IO.FreeBlock(b)
			}
		}
	}

	free := v.GetFreeBlocks()
	var node *Inode
	var inodeBlock int
	var dataBlocks []int
	if exists {
		node = existing
		inodeBlock = node.InodeBlock
		dataBlocks = append([]int(nil), free[:neededData]...)
	} else {
		inodeBlock = free[0]
		dataBlocks = append([]int(nil), free[1:1+neededData]...)
		node = v.CreateInode(false, name)
		node.Parent = parent
		node.InodeBlock = inodeBlock
		parent.Children[name] = node
	}

	if exists && node.Encrypted {
		fmt.Printf("Aviso: '%s' estava criptografado; o novo conteúdo foi gravado em texto claro.\n", name)
	}
	node.Encrypted = false
	node.CryptSalt = ""
	node.KeyCheck = ""

	node.SizeBytes = size
	node.DirectBlocks = dataBlocks
	node.Content = content

	fullFilePath := v.GetFullPath(node)
	cleanName := strings.ToUpper(strings.Split(name, ".")[0])

	v.Blocks[inodeBlock] = &BlockInfo{
		Tag:         fmt.Sprintf("INODE_%s", cleanName),
		Description: fmt.Sprintf("Arquivo '%s'", fullFilePath),
	}

	remainingBytes := size
	for idx, bIdx := range dataBlocks {
		bytesInThisBlock := v.BlockSize
		if remainingBytes < v.BlockSize {
			bytesInThisBlock = remainingBytes
		}
		v.Blocks[bIdx] = &BlockInfo{
			Tag:         "DADO",
			Description: fmt.Sprintf("'%s' (Bloco %d - %d bytes ocupados)", fullFilePath, idx+1, bytesInThisBlock),
		}
		remainingBytes -= bytesInThisBlock
	}

	if v.IO != nil {
		meta := v.inodeMeta(node)
		v.IO.WriteBlock(inodeBlock, meta, fmt.Sprintf("INODE '%s'", fullFilePath))
		v.Sec.RecordBlock(inodeBlock, meta, v.BlockSize)

		payload := buildPayload(content, size, name)
		for idx, bIdx := range dataBlocks {
			start := idx * v.BlockSize
			end := start + v.BlockSize
			if end > len(payload) {
				end = len(payload)
			}
			label := fmt.Sprintf("'%s' - Parte %d/%d", fullFilePath, idx+1, len(dataBlocks))
			v.IO.WriteBlock(bIdx, payload[start:end], label)
			v.Sec.RecordBlock(bIdx, payload[start:end], v.BlockSize)
		}
		v.IO.Flush(fmt.Sprintf("%s %s", cmd, fullFilePath))
	}
}

func (v *VirtualFileSystem) PrintTree(inode *Inode, prefix string) {
	if inode == nil {
		inode = v.Root
		fmt.Println("\n[ÁRVORE DO SISTEMA DE ARQUIVOS]\n/")
	}

	keys := []string{}
	for k := range inode.Children {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for idx, key := range keys {
		child := inode.Children[key]
		isLast := idx == len(keys)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		if child.IsDir {
			fmt.Printf("%s%s%s/ [DIR]\n", prefix, connector, child.Name)
			extension := "│   "
			if isLast {
				extension = "    "
			}
			v.PrintTree(child, prefix+extension)
		} else {
			sizeStr := fmt.Sprintf("%d B", child.SizeBytes)
			if child.SizeBytes >= 1024 {
				sizeStr = fmt.Sprintf("%d KB", child.SizeBytes/1024)
			}
			fmt.Printf("%s%s%s (%s) [ARQ]\n", prefix, connector, child.Name, sizeStr)
		}
	}
}

func (v *VirtualFileSystem) FsStatus(details bool) {
	blockKB := v.BlockSize / 1024
	totalKB := (v.TotalBlocks * v.BlockSize) / 1024
	fmt.Printf("\nTamanho do Bloco de Disco: %d KB | Total de Blocos: %d (%d KB)\n\n", blockKB, v.TotalBlocks, totalKB)
	fmt.Println("[MAPA DE ALOCAÇÃO DO DISCO (Indexado)]")

	i := 0
	for i < v.TotalBlocks {
		if v.Blocks[i] == nil {
			start := i
			for i < v.TotalBlocks && v.Blocks[i] == nil {
				i++
			}
			end := i - 1
			if start == end {
				fmt.Printf("Bloco %02d                  : [LIVRE]\n", start)
			} else {
				fmt.Printf("Bloco %02d..%02d            : [LIVRE]\n", start, end)
			}
		} else {
			blk := v.Blocks[i]
			tagFormatted := fmt.Sprintf("[%s]", blk.Tag)
			fmt.Printf("Bloco %02d %-16s: %s\n", i, tagFormatted, blk.Description)
			i++
		}
	}

	if details {
		fmt.Println("\n--- RELATÓRIO DE FRAGMENTAÇÃO ---")
		ids := []int{}
		for id := range v.Inodes {
			ids = append(ids, id)
		}
		sort.Ints(ids)

		files := []*Inode{}
		for _, id := range ids {
			if node := v.Inodes[id]; !node.IsDir {
				files = append(files, node)
			}
		}

		if len(files) == 0 {
			fmt.Println("Nenhum arquivo alocado no disco.")
			return
		}

		for _, f := range files {
			neededBlocks := (f.SizeBytes + v.BlockSize - 1) / v.BlockSize
			allocatedBytes := neededBlocks * v.BlockSize
			internalFrag := allocatedBytes - f.SizeBytes

			lastBlockBytes := f.SizeBytes % v.BlockSize
			if lastBlockBytes == 0 && f.SizeBytes > 0 {
				lastBlockBytes = v.BlockSize
			}
			wastePct := (float64(v.BlockSize-lastBlockBytes) / float64(v.BlockSize)) * 100

			fmt.Printf("Tamanho Real do Arquivo '%s': %d bytes (%d KB)\n", f.Name, f.SizeBytes, f.SizeBytes/1024)
			fmt.Printf("Espaço Alocado no Disco          : %d bytes (%d blocos de %d KB)\n", allocatedBytes, neededBlocks, blockKB)
			if len(f.DirectBlocks) == 0 {
				fmt.Println("Fragmentação Interna             : 0 bytes (arquivo vazio, sem blocos de dados)")
				continue
			}
			fmt.Printf("Fragmentação Interna             : %d bytes (%.0f%% de desperdício no Bloco %02d)\n", internalFrag, wastePct, f.DirectBlocks[len(f.DirectBlocks)-1])
		}

		fmt.Println("Fragmentação Externa             : 0 bytes")
		fmt.Println("Necessidade de Compactação       : NÃO (Blocos alocados via i-node indexado)")
	}
}

func (v *VirtualFileSystem) Cd(path string) {
	target := v.ResolvePath(path)
	if target == nil {
		fmt.Printf("cd: %s: Diretório não encontrado.\n", path)
		return
	}
	if !target.IsDir {
		fmt.Printf("cd: %s: Não é um diretório.\n", path)
		return
	}
	if !v.CheckAccess(target, security.PermX) {
		return
	}
	v.CurrentDir = target
}

func (v *VirtualFileSystem) Read(path string) {
	v.ReadWithKey(path, "")
}


func (v *VirtualFileSystem) ReadWithKey(path, key string) {
	node := v.ResolvePath(path)
	if node == nil {
		fmt.Printf("read: '%s' não encontrado.\n", path)
		return
	}
	if !v.CheckAccess(node, security.PermR) {
		return
	}
	if node.IsDir {
		names := []string{}
		for name := range node.Children {
			names = append(names, name)
		}
		sort.Strings(names)
		items := []string{}
		for _, name := range names {
			if node.Children[name].IsDir {
				items = append(items, name+"/")
			} else {
				items = append(items, name)
			}
		}
		fmt.Println(strings.Join(items, "  "))
		return
	}

	if v.IO == nil {
		if node.Content != "" {
			fmt.Println(node.Content)
		} else {
			fmt.Printf("[Conteúdo Simulado - Tamanho: %d bytes]\n", node.SizeBytes)
		}
		return
	}

	full := v.GetFullPath(node)
	if node.InodeBlock >= 0 {
		v.IO.ReadBlock(node.InodeBlock, fmt.Sprintf("INODE '%s'", full))
	}
	buf := []byte{}
	for idx, b := range node.DirectBlocks {
		label := fmt.Sprintf("'%s' - Parte %d/%d", full, idx+1, len(node.DirectBlocks))
		buf = append(buf, v.IO.ReadBlock(b, label)...)
	}
	v.IO.Flush(fmt.Sprintf("cat %s", full))

	if len(buf) > node.SizeBytes {
		buf = buf[:node.SizeBytes]
	}
	if node.Encrypted {
		if key == "" {
			fmt.Printf("[ARQUIVO CRIPTOGRAFADO] %s (Encrypted Ciphertext)\n", security.HexPreview(buf, 16))
			fmt.Println("Use: cat <arquivo> --key <chave> para decifrar.")
			return
		}
		plain, err := v.decryptBuffer(node, key, buf)
		if err != nil {
			fmt.Printf("cat: %v\n", err)
			v.Sec.Record("DECIFRAR_FALHOU", false, full)
			return
		}
		buf = plain
	}
	text := strings.ReplaceAll(string(buf), "\x00", "")
	if text == "" {
		fmt.Println("[arquivo vazio]")
		return
	}
	const maxPreview = 160
	runes := []rune(text)
	if len(runes) > maxPreview {
		fmt.Printf("%s...\n[%d bytes no total; exibindo os primeiros %d caracteres]\n", string(runes[:maxPreview]), node.SizeBytes, maxPreview)
	} else {
		fmt.Println(text)
	}
}

func (v *VirtualFileSystem) Delete(path string) {
	node := v.ResolvePath(path)
	if node == nil {
		fmt.Printf("rm: '%s' não encontrado.\n", path)
		return
	}
	if node == v.Root {
		fmt.Println("rm: não é possível remover o diretório raiz.")
		return
	}
	if node.IsDir && len(node.Children) > 0 {
		fmt.Printf("rm: impossível remover '%s': Diretório não vazio.\n", path)
		return
	}
	if !v.CheckAccess(node.Parent, security.PermW) {
		return
	}

	if !node.IsDir {
		freed := append([]int(nil), node.DirectBlocks...)
		if node.InodeBlock >= 0 {
			freed = append(freed, node.InodeBlock)
		}
		for _, b := range freed {
			v.Blocks[b] = nil
			v.Sec.ForgetBlock(b)
			if v.IO != nil {
				v.IO.FreeBlock(b)
			}
		}
	}

	delete(node.Parent.Children, node.Name)
	delete(v.Inodes, node.ID)
}
