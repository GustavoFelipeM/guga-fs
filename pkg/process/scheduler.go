package process

import (
	"fmt"
	"sort"
	"strings"

	"guga-fs/pkg/memory"
)

type ScheduleConfig struct {
	Algorithm string
	Quantum   int
}

func (m *Manager) RunSchedule(cfg ScheduleConfig) {
	if cfg.Quantum <= 0 {
		cfg.Quantum = 2
	}
	if cfg.Algorithm == "" {
		cfg.Algorithm = "Priority_RR"
	}

	all := m.SortedProcs()
	var active []*PCB
	for _, p := range all {
		if p.State != StateTerminated {
			active = append(active, p)
		}
	}
	if len(active) == 0 {
		fmt.Println("schedule: nenhum processo ativo para escalonar. Crie processos com 'create_process' ou use 'schedule --reset'.")
		return
	}

	queues := make(map[int][]*PCB)
	var prioLevels []int
	registerPrio := func(prio int) {
		if _, exists := queues[prio]; !exists {
			queues[prio] = nil
			prioLevels = append(prioLevels, prio)
			sort.Ints(prioLevels)
		}
	}

	for _, p := range active {
		registerPrio(p.Priority)
		if p.State == StateReady || p.State == StateNew {
			p.Transition(StateReady, "")
			queues[p.Priority] = append(queues[p.Priority], p)
		}
	}

	fmt.Printf("\n> schedule --algorithm %s --quantum %dms\n", cfg.Algorithm, cfg.Quantum)
	fmt.Println("Fila de Pronto Inicial:")
	for _, pr := range prioLevels {
		var items []string
		for _, p := range queues[pr] {
			items = append(items, fmt.Sprintf("PID %d (Burst %dms)", p.PID, p.RemainingTime))
		}
		if len(items) > 0 {
			fmt.Printf("- Prioridade %s: [%s]\n", priorityHeading(pr), strings.Join(items, ", "))
		}
	}

	fmt.Println("\n--- LINHA DO TEMPO DA CPU ---")

	var current *PCB
	sliceUsed := 0
	segStart := 0
	contextSwitches := 0
	busyTicks := 0
	totalTicks := 0

	nextReady := func() (*PCB, int) {
		for _, pr := range prioLevels {
			if len(queues[pr]) > 0 {
				p := queues[pr][0]
				queues[pr] = queues[pr][1:]
				return p, pr
			}
		}
		return nil, -1
	}

	highestReadyPrio := func() int {
		for _, pr := range prioLevels {
			if len(queues[pr]) > 0 {
				return pr
			}
		}
		return 1 << 30
	}

	allDone := func() bool {
		for _, p := range active {
			if p.State != StateTerminated {
				return false
			}
		}
		return true
	}

	for !allDone() {
		for _, p := range active {
			if p.State == StateBlocked && m.Clock >= p.BlockedUntil {
				p.Transition(StateReady, "")
				registerPrio(p.Priority)
				queues[p.Priority] = append(queues[p.Priority], p)
				fmt.Printf("[%dms]      Evento E/S: PID %d encerra %s -> Estado %s -> %s\n",
					m.Clock, p.PID, syscallLabel(p), StateBlocked, StateReady)
			}
		}

		if current != nil && strings.EqualFold(cfg.Algorithm, "Priority_RR") {
			if highestReadyPrio() < current.Priority {
				fmt.Printf("[%dms - %dms] Executando PID %d | Preemptado por prioridade maior -> Volta p/ Fila P%d\n",
					segStart, m.Clock, current.PID, current.Priority)
				current.Transition(StateReady, "")
				queues[current.Priority] = append(queues[current.Priority], current)
				current = nil
				sliceUsed = 0
				contextSwitches++
			}
		}

		if current == nil {
			next, _ := nextReady()
			if next != nil {
				current = next
				current.Transition(StateRunning, "")
				if current.FirstRunAt < 0 {
					current.FirstRunAt = m.Clock
				}
				sliceUsed = 0
				segStart = m.Clock
			}
		}

		m.Clock++
		totalTicks++

		if current == nil {
			continue
		}

		busyTicks++
		sliceUsed++
		current.ExecutedTime++
		current.RemainingTime--

		if current.Type == memory.Normal && m.Mem != nil {
			prevQuiet := m.Mem.Quiet
			m.Mem.Quiet = true
			m.Mem.Access(current.MemProc, current.NextLogicalPage(), false)
			m.Mem.Quiet = prevQuiet
		}

		if current.RemainingTime == 0 {
			current.Transition(StateTerminated, "")
			current.CompletionAt = m.Clock
			emptyNote := ""
			if len(queues[current.Priority]) == 0 {
				emptyNote = fmt.Sprintf(" (Fila P%d VAZIA)", current.Priority)
			}
			fmt.Printf("[%dms - %dms] Executando PID %d | PID %d %s!%s\n",
				segStart, m.Clock, current.PID, current.PID, StateTerminated, emptyNote)
			current = nil
			sliceUsed = 0
			if !allDone() {
				contextSwitches++
			}
			continue
		}

		if current.Syscall.Enabled && !current.Syscall.Triggered && current.ExecutedTime >= current.Syscall.TriggerAt {
			current.Syscall.Triggered = true
			current.BlockedUntil = m.Clock + current.Syscall.Duration
			current.Transition(StateBlocked, fmt.Sprintf("E/S (%s)", current.Syscall.Name))
			m.enqueueDiskSyscall(current)

			fmt.Printf("[%dms - %dms] Executando PID %d | Dispara Syscall -> Estado %s (até t=%dms)\n",
				segStart, m.Clock, current.PID, StateBlocked, current.BlockedUntil)
			current = nil
			sliceUsed = 0
			contextSwitches++
			continue
		}

		if sliceUsed >= cfg.Quantum {
			fmt.Printf("[%dms - %dms] Executando PID %d | Quantum %dms expirou -> Volta p/ Fila P%d\n",
				segStart, m.Clock, current.PID, cfg.Quantum, current.Priority)
			current.Transition(StateReady, "")
			queues[current.Priority] = append(queues[current.Priority], current)
			current = nil
			sliceUsed = 0
			contextSwitches++
		}
	}

	m.printMetricsReport(active, contextSwitches, busyTicks, totalTicks)
}

func (m *Manager) printMetricsReport(procs []*PCB, contextSwitches, busyTicks, totalTicks int) {
	sort.Slice(procs, func(i, j int) bool { return procs[i].PID < procs[j].PID })

	fmt.Println("\n================ METRICAS FINAIS DO ESCALONADOR ================")
	fmt.Println("+----------+------------+---------------+-------------------+---------------------+")
	fmt.Println("| Processo | Prioridade | Tempo Chegada | Tempo Espera (WT) | Turnaround Time(TAT)|")
	fmt.Println("+----------+------------+---------------+-------------------+---------------------+")

	var sumWT, sumTAT, sumResp float64
	for _, p := range procs {
		tat := p.CompletionAt - p.ArrivalTime
		if tat < 0 {
			tat = 0
		}
		wt := tat - p.BurstTime
		if wt < 0 {
			wt = 0
		}
		resp := p.FirstRunAt - p.ArrivalTime
		if resp < 0 {
			resp = 0
		}
		sumWT += float64(wt)
		sumTAT += float64(tat)
		sumResp += float64(resp)

		fmt.Printf("| PID %-4d |     %-6d |     %4d ms   |       %4d ms     |        %4d ms      |\n",
			p.PID, p.Priority, p.ArrivalTime, wt, tat)
	}
	fmt.Println("+----------+------------+---------------+-------------------+---------------------+")

	n := float64(len(procs))
	cpuUtil := 0.0
	throughput := 0.0
	if totalTicks > 0 {
		cpuUtil = float64(busyTicks) * 100.0 / float64(totalTicks)
		throughput = n / float64(totalTicks)
	}

	fmt.Printf("\n- Tempo Médio de Espera (AWT)   : %.2f ms\n", sumWT/n)
	fmt.Printf("- Tempo Médio de Retorno (ATT)  : %.2f ms\n", sumTAT/n)
	fmt.Printf("- Tempo Médio de Resposta (ART) : %.2f ms\n", sumResp/n)
	fmt.Printf("- Trocas de Contexto Totais     : %d trocas\n", contextSwitches)
	fmt.Printf("- Utilização Global da CPU      : %.0f%%\n", cpuUtil)
	fmt.Printf("- Vazão (Throughput)            : %.3f processos/ms\n", throughput)
	fmt.Println("========================================================================")
}

func (m *Manager) ResetForReschedule() int {
	count := 0
	m.Clock = 0
	for _, p := range m.SortedProcs() {
		p.State = StateReady
		p.RemainingTime = p.BurstTime
		p.ExecutedTime = 0
		p.BlockedUntil = 0
		p.BlockReason = ""
		p.CompletionAt = -1
		p.FirstRunAt = -1
		p.Syscall.Triggered = false
		count++
	}
	return count
}

func priorityHeading(p int) string {
	if p <= 1 {
		return fmt.Sprintf("%d (Alta) ", p)
	}
	return fmt.Sprintf("%d (Baixa)", p)
}

func syscallLabel(p *PCB) string {
	if strings.Contains(strings.ToLower(p.Syscall.Name), "write") {
		return "escrita"
	}
	return "leitura"
}
