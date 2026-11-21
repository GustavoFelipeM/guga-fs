package commands

import (
	"fmt"
	"strconv"
	"strings"

	"guga-fs/pkg/fs"
	"guga-fs/pkg/memory"
	"guga-fs/pkg/process"
)

func getProcMgr(vfs *fs.VirtualFileSystem) *process.Manager {
	pm, ok := vfs.Proc.(*process.Manager)
	if !ok || pm == nil {
		fmt.Println("Processos: gerenciador não inicializado.")
		return nil
	}
	return pm
}

func parseMsInt(s string) (int, bool) {
	t := strings.ToLower(strings.TrimSpace(s))
	t = strings.TrimSuffix(t, "ms")
	v, err := strconv.Atoi(strings.TrimSpace(t))
	return v, err == nil
}

func HandleRunSimulation(vfs *fs.VirtualFileSystem, args []string) {
	pm := getProcMgr(vfs)
	if pm == nil || !requireLogin(vfs) {
		return
	}
	const usage = "Uso: run_simulation --process <nome|PID> [--steps <N>] [--pages <p1,p2,...>]"
	f := parseFlags(args)
	ref := f.get("process")
	if ref == "" {
		fmt.Println(usage)
		return
	}
	pcb := pm.Lookup(ref)
	if pcb == nil {
		fmt.Printf("run_simulation: processo '%s' não encontrado.\n", ref)
		return
	}
	if !canTouch(vfs, pcb.MemProc, "run_simulation") {
		return
	}

	steps := 3
	if f.isSet("steps") {
		n, ok := parseInt(f.get("steps"))
		if !ok || n <= 0 {
			fmt.Println("run_simulation: --steps deve ser um inteiro positivo.")
			return
		}
		steps = n
	}

	var pages []int
	if f.isSet("pages") {
		for _, tok := range strings.Split(f.get("pages"), ",") {
			if n, ok := parseInt(tok); ok {
				pages = append(pages, n)
			}
		}
	}

	pm.RunSimulation(pcb, steps, pages)
	vfs.Sec.Record("RUN_SIMULATION", true, fmt.Sprintf("%s pid=%d steps=%d", pcb.Name, pcb.PID, steps))
}

func HandleSchedule(vfs *fs.VirtualFileSystem, args []string) {
	pm := getProcMgr(vfs)
	if pm == nil || !requireLogin(vfs) {
		return
	}
	f := parseFlags(args)
	if f.isSet("reset") {
		n := pm.ResetForReschedule()
		fmt.Printf("schedule: %d processo(s) reiniciado(s) para o estado PRONTO (relógio zerado).\n", n)
		if !f.isSet("quantum") && !f.isSet("algorithm") {
			return
		}
	}

	algo := f.get("algorithm")
	if algo == "" {
		algo = "Priority_RR"
	}
	quantum := 2
	if f.isSet("quantum") {
		q, ok := parseMsInt(f.get("quantum"))
		if !ok || q <= 0 {
			fmt.Println("schedule: --quantum inválido (ex.: --quantum 2ms).")
			return
		}
		quantum = q
	}

	pm.RunSchedule(process.ScheduleConfig{
		Algorithm: algo,
		Quantum:   quantum,
	})
	vfs.Sec.Record("SCHEDULE", true, fmt.Sprintf("algo=%s quantum=%dms", algo, quantum))
}

func HandlePs(vfs *fs.VirtualFileSystem, args []string) {
	pm := getProcMgr(vfs)
	if pm == nil {
		return
	}
	procs := pm.SortedProcs()
	if len(procs) == 0 {
		fmt.Println("Nenhum processo cadastrado.")
		return
	}
	fmt.Println("\n+------+------------------+------------+------------+-----------+----------+----------------------------------------+")
	fmt.Println("| PID  | Nome             | Tipo       | Estado     | Prioridade| Burst    | Syscall / E/S                          |")
	fmt.Println("+------+------------------+------------+------------+-----------+----------+----------------------------------------+")
	for _, p := range procs {
		burstInfo := fmt.Sprintf("%d/%dms", p.RemainingTime, p.BurstTime)
		fmt.Printf("| %-4d | %-16s | %-10s | %-10s | %-9d | %-8s | %-38s |\n",
			p.PID, p.Name, p.Type.String(), p.State, p.Priority, burstInfo, p.Syscall.Summary())
	}
	fmt.Println("+------+------------------+------------+------------+-----------+----------+----------------------------------------+")
}

func HandleCreateThread(vfs *fs.VirtualFileSystem, args []string) {
	pm := getProcMgr(vfs)
	if pm == nil || !requireLogin(vfs) {
		return
	}
	f := parseFlags(args)
	ref := f.get("process")
	if ref == "" {
		fmt.Println("Uso: create_thread --process <nome|PID> [--name <T2>]")
		return
	}
	pcb := pm.Lookup(ref)
	if pcb == nil {
		fmt.Printf("create_thread: processo '%s' não encontrado.\n", ref)
		return
	}
	t := pcb.AddThread(f.get("name"))
	fmt.Printf("[THREAD CRIADA] TID %d (%s) no processo %s (PID %d) | Estado: %s\n",
		t.TID, t.Name, pcb.Name, pcb.PID, t.State)
}

func HandleMutexLock(vfs *fs.VirtualFileSystem, args []string) {
	pm := getProcMgr(vfs)
	if pm == nil || !requireLogin(vfs) {
		return
	}
	f := parseFlags(args)
	ref, th, mx := f.get("process"), f.get("thread"), f.get("mutex")
	if ref == "" || th == "" || mx == "" {
		fmt.Println("Uso: mutex_lock --process <nome|PID> --thread <nome> --mutex <recurso> [--time <Nms>]")
		return
	}
	pcb := pm.Lookup(ref)
	if pcb == nil {
		fmt.Printf("mutex_lock: processo '%s' não encontrado.\n", ref)
		return
	}
	at := pm.Clock
	if f.isSet("time") {
		if t, ok := parseMsInt(f.get("time")); ok {
			at = t
		}
	}
	pm.LockMutex(pcb, th, mx, at)
}

func HandleMutexUnlock(vfs *fs.VirtualFileSystem, args []string) {
	pm := getProcMgr(vfs)
	if pm == nil || !requireLogin(vfs) {
		return
	}
	f := parseFlags(args)
	ref, th, mx := f.get("process"), f.get("thread"), f.get("mutex")
	if ref == "" || th == "" || mx == "" {
		fmt.Println("Uso: mutex_unlock --process <nome|PID> --thread <nome> --mutex <recurso> [--time <Nms>]")
		return
	}
	pcb := pm.Lookup(ref)
	if pcb == nil {
		fmt.Printf("mutex_unlock: processo '%s' não encontrado.\n", ref)
		return
	}
	at := pm.Clock
	if f.isSet("time") {
		if t, ok := parseMsInt(f.get("time")); ok {
			at = t
		}
	}
	pm.UnlockMutex(pcb, th, mx, at)
}

func HandleConcurrencyDemo(vfs *fs.VirtualFileSystem, args []string) {
	pm := getProcMgr(vfs)
	if pm == nil || !requireLogin(vfs) {
		return
	}
	pcb := pm.Lookup("102")
	if pcb == nil {
		pcb = pm.Lookup("App_Render")
	}
	if pcb == nil {
		var err error
		pcb, err = pm.Create(process.CreateOptions{
			Name:     "App_Render",
			Owner:    vfs.Sec.CurrentName(),
			Size:     32 * memory.KB,
			Type:     memory.Normal,
			Burst:    5,
			Priority: 2,
		})
		if err != nil {
			fmt.Println("concurrency_demo:", err)
			return
		}
	}
	if pcb.FindThread("T2") == nil {
		pcb.AddThread("T2")
	}

	fmt.Printf("\n[CONCORRÊNCIA DE THREADS EM PID %d]\n", pcb.PID)
	pm.LockMutex(pcb, "T1", "Buffer_Video", 4)
	pm.LockMutex(pcb, "T2", "Buffer_Video", 5)
	pm.UnlockMutex(pcb, "T1", "Buffer_Video", 6)
}
