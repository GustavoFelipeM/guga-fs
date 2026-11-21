package commands

import (
	"fmt"
	"strconv"

	"guga-fs/pkg/fs"
	"guga-fs/pkg/storage"
)

const raidUsage = `Uso: raid [comando]
  raid | raid status          mostra o arranjo e as estatísticas de E/S
  raid <0|1|4|5> [--disks N]  reconfigura o arranjo (migra os dados atuais)
  raid fail <disco>           força a falha de um disco (1..N)
  raid rebuild <disco>        substitui o disco falho e reconstrói via redundância
  raid test <disco>           simula falha, reconstrói cada bloco e confere`

func HandleRaid(vfs *fs.VirtualFileSystem, args []string) {
	ctrl := getController(vfs)
	if ctrl == nil {
		return
	}

	if len(args) == 0 || args[0] == "status" {
		ctrl.Status()
		return
	}

	if !vfs.Sec.RequireKernel("raid " + args[0]) {
		return
	}

	switch args[0] {
	case "fail", "rebuild", "test":
		if len(args) < 2 {
			fmt.Println(raidUsage)
			return
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 || n > len(ctrl.Array.Disks) {
			fmt.Printf("raid: disco inválido (use 1..%d)\n", len(ctrl.Array.Disks))
			return
		}
		d := n - 1
		switch args[0] {
		case "fail":
			ctrl.Array.Fail(d)
			fmt.Printf("* Falha forçada no Disco %d!\n", n)
			if ctrl.Array.Level == storage.RAID0 {
				fmt.Println("  Atenção: RAID 0 não tem redundância; leituras nesse disco falharão.")
			}
		case "rebuild":
			if err := ctrl.Array.Rebuild(d); err != nil {
				fmt.Printf("raid: %v\n", err)
				return
			}
			fmt.Printf("* Disco %d substituído e reconstruído via redundância [OK]\n", n)
		case "test":
			ctrl.FailureTest(d)
		}
		return
	}

	level, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Println(raidUsage)
		return
	}
	disks := 3
	if level == 0 || level == 1 {
		disks = 2
	}
	for i := 1; i < len(args); i++ {
		if args[i] == "--disks" && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				fmt.Println("raid: número de discos inválido")
				return
			}
			disks = n
			i++
		}
	}
	if err := ctrl.Reconfigure(storage.RAIDLevel(level), disks); err != nil {
		fmt.Printf("raid: %v\n", err)
		return
	}
	fmt.Printf("Arranjo reconfigurado: RAID %d com %d discos (dados existentes migrados).\n", level, disks)
}
