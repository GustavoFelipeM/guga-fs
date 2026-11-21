package process

import (
	"fmt"
	"sort"

	"guga-fs/pkg/memory"
)

type frameSnapshot struct {
	Frame     int
	PID       int
	PageIndex int
	Status    string
	LastLRU   int
}

func (m *Manager) RunSimulation(p *PCB, steps int, explicitPages []int) {
	if steps <= 0 {
		steps = 3
	}
	if p.State == StateTerminated {
		fmt.Printf("run_simulation: processo '%s' (PID %d) já está %s.\n", p.Name, p.PID, p.State)
		return
	}
	if p.State == StateBlocked && m.Clock < p.BlockedUntil {
		fmt.Printf("run_simulation: processo '%s' (PID %d) está %s até t=%dms (%s).\n",
			p.Name, p.PID, p.State, p.BlockedUntil, p.BlockReason)
		return
	}

	p.Transition(StateRunning, "")
	snapshots := map[int]frameSnapshot{}

	for s := 1; s <= steps; s++ {
		if p.State == StateBlocked || p.State == StateTerminated {
			break
		}

		m.Clock++
		p.ExecutedTime++
		if p.RemainingTime > 0 {
			p.RemainingTime--
		}

		var pageIdx int
		if len(explicitPages) > 0 {
			pageIdx = explicitPages[(s-1)%len(explicitPages)]
		} else {
			pageIdx = p.NextLogicalPage()
		}

		fmt.Printf("\n--- STEP %d (t=%dms) | Executando PID %d (%s) ---\n", s, m.Clock, p.PID, p.Name)

		if p.Type == memory.Critical {
			fmt.Printf("CPU acessa: Área Reservada [0x%X] -> [ACESSO DIRETO CONTÍGUO] (t=%dms)\n", p.MemProc.Base, m.Clock)
		} else {
			m.accessPageWithTrace(p, pageIdx, snapshots)
		}

		if p.Syscall.Enabled && !p.Syscall.Triggered && p.ExecutedTime >= p.Syscall.TriggerAt {
			p.Syscall.Triggered = true
			p.BlockedUntil = m.Clock + p.Syscall.Duration
			p.Transition(StateBlocked, fmt.Sprintf("E/S (%s)", p.Syscall.Name))
			track := m.enqueueDiskSyscall(p)

			fmt.Printf("\n--- CONTINUAÇÃO DA EXECUÇÃO (t=%dms) ---\n", m.Clock)
			fmt.Printf("[SYSCALL DETECTADA]: PID %d atingiu a instrução '%s' agendada para t=%dms!\n",
				p.PID, p.Syscall.Name, p.Syscall.TriggerAt)
			fmt.Printf("- Mudança de Estado: %s -> %s (E/S de Disco).\n", StateRunning, StateBlocked)
			if track >= 0 {
				fmt.Printf("- Requisição de disco enfileirada na trilha %d (Controlador Etapa 2).\n", track)
			}
			fmt.Printf("- Tempo de Retorno Previsto do Bloqueio: %dms (Desbloqueio em t=%dms).\n",
				p.Syscall.Duration, p.BlockedUntil)
			fmt.Printf("- CPU liberada -> Escalonador seleciona próximo processo da Fila de Pronto.\n")
			break
		}

		if p.RemainingTime == 0 {
			p.Transition(StateTerminated, "")
			p.CompletionAt = m.Clock
			fmt.Printf("[CONCLUÍDO] PID %d (%s) finalizou seu burst de CPU em t=%dms.\n", p.PID, p.Name, m.Clock)
			break
		}
	}

	if p.State == StateRunning {
		p.Transition(StateReady, "")
	}

	if p.Type == memory.Normal && len(snapshots) > 0 {
		printRAMSummaryTable(snapshots)
	}
}

func (m *Manager) accessPageWithTrace(p *PCB, pageIdx int, snapshots map[int]frameSnapshot) {
	mem := m.Mem
	if pageIdx < 0 || pageIdx >= len(p.MemProc.Pages) {
		fmt.Printf("[VIOLAÇÃO DE ENDEREÇO] Página Lógica %02d fora dos limites de %s.\n", pageIdx, p.Name)
		return
	}

	pg := p.MemProc.Pages[pageIdx]
	wasPresent := pg.Present

	var victimDesc string
	if !wasPresent {
		hasFree := false
		for _, f := range mem.Frames {
			if f == nil {
				hasFree = true
				break
			}
		}
		if !hasFree && len(mem.Frames) > 0 {
			victim := mem.Frames[0]
			for _, f := range mem.Frames[1:] {
				if f != nil && f.LastUsed < victim.LastUsed {
					victim = f
				}
			}
			if victim != nil {
				idleMs := (mem.Clock - victim.LastUsed) * memory.MsPerTick
				if idleMs <= 0 {
					idleMs = memory.MsPerTick
				}
				victimDesc = fmt.Sprintf("Página Lógica %02d do PID %d (não usada há %dms) removida para o Swap.",
					victim.Index, victim.Proc.PID, idleMs)
			}
		}
	}

	prevQuiet := mem.Quiet
	mem.Quiet = true
	mem.Access(p.MemProc, pageIdx, false)
	mem.Quiet = prevQuiet

	if wasPresent {
		fmt.Printf("CPU acessa: Página Lógica %02d -> [RAM: Moldura %02d - HIT] | Histórico %s: Pág %02d atualizada (t=%dms)\n",
			pageIdx, pg.Frame, mem.Algo.Short(), pageIdx, m.Clock)
		snapshots[pg.Frame] = frameSnapshot{
			Frame:     pg.Frame,
			PID:       p.PID,
			PageIndex: pageIdx,
			Status:    "HIT",
			LastLRU:   m.Clock,
		}
	} else {
		fmt.Printf("CPU acessa: Página Lógica %02d -> [PAGE FAULT!]\n", pageIdx)
		if victimDesc != "" {
			fmt.Printf("Substituição %s: %s\n", mem.Algo.Short(), victimDesc)
		}
		fmt.Printf("Página Lógica %02d alocada na Moldura %02d.\n", pageIdx, pg.Frame)
		snapshots[pg.Frame] = frameSnapshot{
			Frame:     pg.Frame,
			PID:       p.PID,
			PageIndex: pageIdx,
			Status:    "PAGE FAULT (NEW)",
			LastLRU:   m.Clock,
		}
	}
}

func printRAMSummaryTable(snapshots map[int]frameSnapshot) {
	keys := make([]int, 0, len(snapshots))
	for k := range snapshots {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	fmt.Println("\n[RESUMO DA GERÊNCIA DE MEMÓRIA (RAM DINÂMICA)]")
	fmt.Println("+------------------+---------------+-------------------+---------------------+")
	fmt.Println("| Moldura RAM (F)  | Processo/Pág  | Estado de Acesso  | Último Acesso (LRU) |")
	fmt.Println("+------------------+---------------+-------------------+---------------------+")
	for _, k := range keys {
		s := snapshots[k]
		fmt.Printf("|    Moldura %02d    | PID %-3d / P%-2d | %-17s | t = %-3d ms          |\n",
			s.Frame, s.PID, s.PageIndex, s.Status, s.LastLRU)
	}
	fmt.Println("+------------------+---------------+-------------------+---------------------+")
}
