package fs

import (
	"fmt"
	"strings"
)

func (v *VirtualFileSystem) Mkdir(path string) {
	parts := []string{}
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		fmt.Println("mkdir: caminho inválido")
		return
	}

	dirName := parts[len(parts)-1]
	parentPath := "/" + strings.Join(parts[:len(parts)-1], "/")
	if !strings.HasPrefix(path, "/") {
		if len(parts) > 1 {
			parentPath = strings.Join(parts[:len(parts)-1], "/")
		} else {
			parentPath = "."
		}
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

	newDir := v.CreateInode(true, dirName)
	newDir.Parent = parent
	parent.Children[dirName] = newDir
}

func (v *VirtualFileSystem) Create(path, sizeStr string) {
	sizeBytes := ParseSize(sizeStr)
	parts := []string{}
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		fmt.Println("create: caminho inválido")
		return
	}

	fileName := parts[len(parts)-1]
	parentPath := "/" + strings.Join(parts[:len(parts)-1], "/")
	if !strings.HasPrefix(path, "/") {
		if len(parts) > 1 {
			parentPath = strings.Join(parts[:len(parts)-1], "/")
		} else {
			parentPath = "."
		}
	}

	parent := v.ResolvePath(parentPath)
	if parent == nil || !parent.IsDir {
		fmt.Println("create: diretório pai não encontrado.")
		return
	}
	if _, exists := parent.Children[fileName]; exists {
		fmt.Printf("create: arquivo '%s' já existe.\n", fileName)
		return
	}

	neededDataBlocks := (sizeBytes + v.BlockSize - 1) / v.BlockSize
	neededTotalBlocks := 1 + neededDataBlocks

	freeBlocks := v.GetFreeBlocks()
	if len(freeBlocks) < neededTotalBlocks {
		fmt.Printf("Erro: Espaço insuficiente (%d blocos necessários, %d livres).\n", neededTotalBlocks, len(freeBlocks))
		return
	}

	inodeBlock := freeBlocks[0]
	dataBlocks := freeBlocks[1 : 1+neededTotalBlocks-1]

	fInode := v.CreateInode(false, fileName)
	fInode.SizeBytes = sizeBytes
	fInode.Parent = parent
	fInode.DirectBlocks = dataBlocks
	parent.Children[fileName] = fInode

	fullFilePath := v.GetFullPath(fInode)
	cleanName := strings.ToUpper(strings.Split(fileName, ".")[0])

	v.Blocks[inodeBlock] = &BlockInfo{
		Tag:         fmt.Sprintf("INODE_%s", cleanName),
		Description: fmt.Sprintf("Arquivo '%s'", fullFilePath),
	}

	remainingBytes := sizeBytes
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
		files := []*Inode{}
		for _, node := range v.Inodes {
			if !node.IsDir {
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
			fmt.Printf("Fragmentação Interna             : %d bytes (%.0f%% de desperdício no Bloco %02d)\n", internalFrag, wastePct, f.DirectBlocks[len(f.DirectBlocks)-1])
		}

		fmt.Println("Fragmentação Externa             : 0 bytes")
		fmt.Println("Necessidade de Compactação       : NÃO (Blocos alocados via i-node indexado)")
	}
}
