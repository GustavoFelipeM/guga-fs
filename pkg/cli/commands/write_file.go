package commands

import (
	"fmt"
	"strconv"
	"strings"

	"guga-fs/pkg/fs"
)

const writeFileUsage = `Uso: write_file <caminho> ["conteúdo"] [--size <tamanho>] [--fail <disco>]
  Ex.: write_file /sistema/logs/app.log "Novo Registro"
       write_file /dados/planilha.csv --size 8KB --fail 2`

func HandleWriteFile(vfs *fs.VirtualFileSystem, args []string) {
	if len(args) < 1 {
		fmt.Println(writeFileUsage)
		return
	}

	path := args[0]
	words := []string{}
	size := ""
	failDisk := 0

	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--size":
			if i+1 >= len(rest) {
				fmt.Println(writeFileUsage)
				return
			}
			size = rest[i+1]
			i++
		case "--fail":
			if i+1 >= len(rest) {
				fmt.Println(writeFileUsage)
				return
			}
			n, err := strconv.Atoi(rest[i+1])
			if err != nil || n < 1 {
				fmt.Println("write_file: número de disco inválido em --fail")
				return
			}
			failDisk = n
			i++
		default:
			words = append(words, rest[i])
		}
	}

	vfs.WriteFile(path, strings.Join(words, " "), size)

	if failDisk > 0 {
		if ctrl := getController(vfs); ctrl != nil {
			ctrl.FailureTest(failDisk - 1)
		}
	}
}
