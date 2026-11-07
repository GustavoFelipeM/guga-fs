package commands

import (
	"fmt"
	"strconv"

	"guga-fs/pkg/fs"
	"guga-fs/pkg/storage"
)

const ioschedUsage = `Uso: iosched [comando]
  iosched                       mostra a configuração atual do disco
  iosched <fcfs|sstf|scan|cscan>  escolhe o algoritmo de escalonamento
  iosched head <trilha>         posiciona a cabeça de leitura
  iosched dir <up|down>         sentido inicial do braço (SCAN / C-SCAN)
  iosched queue add <trilhas..> adiciona requisições de outros processos
  iosched queue show|clear      mostra / limpa essas requisições
  iosched demo                  executa o exemplo 98,183,37,122,14 (cabeça em 53)`

func HandleIOSched(vfs *fs.VirtualFileSystem, args []string) {
	ctrl := getController(vfs)
	if ctrl == nil {
		return
	}

	if len(args) == 0 {
		dir := "subindo"
		if !ctrl.Up {
			dir = "descendo"
		}
		fmt.Printf("Disco: %d trilhas (0..%d) | Algoritmo: %s | Cabeça: trilha %d | Sentido: %s\n",
			ctrl.Tracks, ctrl.Tracks-1, ctrl.Algo, ctrl.Head, dir)
		fmt.Printf("Requisições de fundo na fila: %v | Deslocamento acumulado: %d trilhas\n",
			ctrl.Background, ctrl.TotalMoved)
		return
	}

	if algo, ok := storage.ParseAlgorithm(args[0]); ok {
		ctrl.Algo = algo
		fmt.Printf("Algoritmo de escalonamento: %s\n", algo)
		return
	}

	switch args[0] {
	case "head":
		if len(args) < 2 {
			fmt.Println(ioschedUsage)
			return
		}
		t, err := strconv.Atoi(args[1])
		if err != nil || t < 0 || t >= ctrl.Tracks {
			fmt.Printf("iosched: trilha inválida (use 0..%d)\n", ctrl.Tracks-1)
			return
		}
		ctrl.Head = t
		fmt.Printf("Cabeça de leitura posicionada na trilha %d\n", t)

	case "dir":
		if len(args) < 2 || (args[1] != "up" && args[1] != "down") {
			fmt.Println("Uso: iosched dir <up|down>")
			return
		}
		ctrl.Up = args[1] == "up"
		fmt.Printf("Sentido inicial do braço: %s\n", args[1])

	case "queue":
		handleQueue(ctrl, args[1:])

	case "demo":
		runDemo(ctrl)

	default:
		fmt.Println(ioschedUsage)
	}
}

func handleQueue(ctrl *storage.Controller, args []string) {
	if len(args) == 0 {
		fmt.Println(ioschedUsage)
		return
	}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			fmt.Println("Uso: iosched queue add <trilha> [trilha ...]")
			return
		}
		for _, a := range args[1:] {
			t, err := strconv.Atoi(a)
			if err != nil || t < 0 || t >= ctrl.Tracks {
				fmt.Printf("iosched: trilha '%s' inválida (use 0..%d)\n", a, ctrl.Tracks-1)
				return
			}
		}
		for _, a := range args[1:] {
			t, _ := strconv.Atoi(a)
			ctrl.Background = append(ctrl.Background, t)
		}
		fmt.Printf("Requisições pendentes (outros processos): %v\n", ctrl.Background)
	case "show":
		fmt.Printf("Requisições pendentes (outros processos): %v\n", ctrl.Background)
	case "clear":
		ctrl.Background = nil
		fmt.Println("Fila de requisições de fundo limpa.")
	default:
		fmt.Println(ioschedUsage)
	}
}

// runDemo reproduz o Exemplo 2.1 do enunciado com todos os algoritmos.
func runDemo(ctrl *storage.Controller) {
	queue := []int{98, 183, 37, 122, 14}
	head := 53
	fmt.Printf("\n[E/S] Requisições de leitura/escrita pendentes nas trilhas: %v\n", queue)
	fmt.Printf("Posição atual da cabeça de leitura: Trilha %d\n", head)
	for _, a := range storage.AllAlgorithms() {
		storage.Schedule(a, head, queue, ctrl.Tracks-1, ctrl.Up).Print()
	}
	storage.Compare(head, queue, ctrl.Tracks-1, ctrl.Up)
}
