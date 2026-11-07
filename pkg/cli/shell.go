package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"guga-fs/pkg/cli/commands"
	"guga-fs/pkg/fs"
)

func RunShell(vfs *fs.VirtualFileSystem) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\033[1;36m=== Virtual File System Terminal (Modular) v1.0 ===\033[0m")
	fmt.Println("Digite 'help' para comandos. Digite 'exit' para encerrar.")

	for {
		cwd := vfs.GetFullPath(nil)
		fmt.Printf("\033[1;32mguga@vfs\033[0m:\033[1;34m%s\033[0m$ ", cwd)

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmdName := strings.ToLower(parts[0])
		args := parts[1:]

		if cmdName == "exit" || cmdName == "quit" {
			fmt.Println("Sessão encerrada.")
			return
		}

		if cmdName == "help" {
			fmt.Println("Comandos disponíveis no registro:")
			for name := range commands.Registry {
				fmt.Printf(" - %s\n", name)
			}
			continue
		}

		handler, exists := commands.Registry[cmdName]
		if exists {
			handler(vfs, args)
		} else {
			fmt.Printf("Comando '%s' não encontrado. Digite 'help' para listar.\n", cmdName)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao ler entrada do terminal: %v\n", err)
	}
}
