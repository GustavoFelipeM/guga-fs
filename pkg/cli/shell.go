package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"guga-fs/pkg/cli/commands"
	"guga-fs/pkg/fs"
	"guga-fs/pkg/security"
)

func setEcho(on bool) {
	arg := "-echo"
	if on {
		arg = "echo"
	}
	cmd := exec.Command("stty", arg)
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

var commandsOpenToGuests = map[string]bool{
	"login": true,
	"help":  true,
	"clear": true,
}

func RunShell(vfs *fs.VirtualFileSystem) {
	scanner := bufio.NewScanner(os.Stdin)

	if vfs.Sec == nil {
		vfs.Sec = security.NewSystem()
	}
	vfs.Sec.ReadSecret = func(prompt string) string {
		fmt.Print(prompt)
		setEcho(false)
		ok := scanner.Scan()
		setEcho(true)
		line := ""
		if ok {
			line = strings.TrimRight(scanner.Text(), "\r\n")
		}
		fmt.Println(strings.Repeat("*", len(line)))
		return line
	}

	fmt.Println("\033[1;36m=== Virtual File System Terminal (Modular) v3.0 — Segurança e Proteção ===\033[0m")
	fmt.Println("Digite 'help' para comandos. Digite 'exit' para encerrar.")
	fmt.Printf("Faça login para começar: login --user %s (senha inicial: %s)\n", security.RootUser, security.DefaultRootPassword)

	for {
		user, symbol := "guest", "$"
		if vfs.Sec.Current != nil {
			user = vfs.Sec.Current.Name
			if vfs.Sec.IsKernel() {
				symbol = "#"
			}
		}
		cwd := vfs.GetFullPath(nil)
		fmt.Printf("\033[1;32m%s@vfs\033[0m:\033[1;34m%s\033[0m%s ", user, cwd, symbol)

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
			names := make([]string, 0, len(commands.Registry))
			for name := range commands.Registry {
				names = append(names, name)
			}
			sort.Strings(names)
			fmt.Println("Comandos disponíveis no registro:")
			for _, name := range names {
				fmt.Printf(" - %s\n", name)
			}
			continue
		}

		if vfs.Sec.Current == nil && !commandsOpenToGuests[cmdName] {
			fmt.Printf("Acesso negado: nenhum usuário autenticado. Use: login --user <nome>\n")
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
