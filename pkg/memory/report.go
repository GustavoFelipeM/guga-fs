package memory

import (
	"fmt"
	"strconv"
	"strings"
)

func bar(used, total int) string {
	const w = 20
	n := 0
	if total > 0 {
		n = used * w / total
	}
	if n > w {
		n = w
	}
	return "[" + strings.Repeat("#", n) + strings.Repeat("-", w-n) + "]"
}

func pct(used, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) * 100 / float64(total)
}

// ---------- Área Reservada ----------

func (m *Manager) PrintReserved() {
	total := fmtMB(m.ReservedSize)
	if m.ReservedSize%MB == 0 {
		total = fmt.Sprintf("%d MB", m.ReservedSize/MB)
	}
	fmt.Printf("\n--- MAPA DA ÁREA CRÍTICA/RESERVADA DA RAM (Total: %s - Alocação Fixa) ---\n", total)
	m.printBlocks()

	ef := m.ExternalFrag()
	fmt.Printf("\n--- DIAGNÓSTICO DE FRAGMENTAÇÃO EXTERNA ---\n")
	fmt.Printf("Total de Memória Livre na Área Reservada : %s (%s)\n", fmtKB(ef.TotalFree), fmtMB(ef.TotalFree))
	fmt.Printf("Maior Bloco Contíguo Disponível          : %s (%s)\n", fmtKB(ef.Largest), fmtMB(ef.Largest))
	switch {
	case ef.Present():
		fmt.Printf("Fragmentação Externa Detectada           : SIM (%d fragmentos não contíguos)\n", ef.Holes)
		fmt.Printf("Índice de Fragmentação Externa           : %.1f%% (1 - maior bloco / total livre)\n", ef.Index())
		fmt.Printf("Status: Uma requisição contígua maior que %s (e de até %s) falhará por ausência de bloco único.\n",
			fmtKB(ef.Largest), fmtKB(ef.TotalFree))
		fmt.Println("        Use 'memory_compact --reserved' para unir as lacunas.")
	case ef.Holes == 1:
		fmt.Printf("Fragmentação Externa Detectada           : NÃO (1 bloco livre contíguo)\n")
		fmt.Printf("Status: Qualquer requisição contígua de até %s será atendida.\n", fmtKB(ef.Largest))
	default:
		fmt.Printf("Fragmentação Externa Detectada           : NÃO (Área Reservada totalmente ocupada)\n")
	}
}

// ---------- RAM dinâmica / memória virtual ----------

func (m *Manager) PrintVirtual() {
	fmt.Printf("\n--- MAPA DA RAM DINÂMICA (Paginada: %d molduras x %d KB = %s | Algoritmo: %s) ---\n",
		m.NumFrames, PageSize/KB, fmtKB(m.NumFrames*PageSize), m.Algo.Label())
	var free []int
	for i, pg := range m.Frames {
		if pg == nil {
			free = append(free, i)
			continue
		}
		fmt.Printf("Moldura %02d : PID %d %-18s Página %d  [R=%d M=%d]  último acesso há %d ms\n",
			i, pg.Proc.PID, pg.Proc.Name, pg.Index, b2i(pg.Referenced), b2i(pg.Dirty), (m.Clock-pg.LastUsed)*MsPerTick)
	}
	if len(free) > 0 {
		fmt.Printf("Molduras livres (%d): %s\n", len(free), rangesText(free))
	}

	fmt.Printf("\n--- TABELAS DE PÁGINAS ---\n")
	procs := m.normalProcs()
	if len(procs) == 0 {
		fmt.Println("(nenhum processo normal ativo)")
	}
	for _, p := range procs {
		fmt.Printf("PID %d %s (dono: %s | %s | %d página(s))\n", p.PID, p.Name, p.Owner, fmtKB(p.Size), len(p.Pages))
		compact := len(p.Pages) > 32
		hidden := 0
		for _, pg := range p.Pages {
			var state string
			switch {
			case pg.Present && pg.SwapSlot >= 0:
				state = fmt.Sprintf("Moldura %d [PRESENTE] (cópia no Swap, slot %d)", pg.Frame, pg.SwapSlot)
			case pg.Present:
				state = fmt.Sprintf("Moldura %d [PRESENTE]", pg.Frame)
			case pg.SwapSlot >= 0:
				state = fmt.Sprintf("- [NO SWAP, slot %d]", pg.SwapSlot)
			default:
				if compact {
					hidden++
					continue
				}
				state = "- [AINDA NÃO CARREGADA]"
			}
			fmt.Printf("  Página %d -> %s | usa %d de %d bytes\n", pg.Index, state, pg.UsedBytes, PageSize)
		}
		if hidden > 0 {
			fmt.Printf("  (%d página(s) ainda não carregadas omitidas)\n", hidden)
		}
	}

	m.PrintInternalFrag()
	m.PrintStats()
}

type IntFrag struct {
	Pages      int
	Underused  int
	WastedByte int
}

func (m *Manager) InternalFrag() IntFrag {
	var f IntFrag
	for _, p := range m.normalProcs() {
		for _, pg := range p.Pages {
			f.Pages++
			if pg.UsedBytes < PageSize {
				f.Underused++
				f.WastedByte += PageSize - pg.UsedBytes
			}
		}
	}
	return f
}

func (m *Manager) PrintInternalFrag() {
	fmt.Printf("\n--- FRAGMENTAÇÃO INTERNA (páginas subutilizadas) ---\n")
	f := m.InternalFrag()
	for _, p := range m.normalProcs() {
		for _, pg := range p.Pages {
			if pg.UsedBytes < PageSize {
				fmt.Printf("PID %d %s: Página %d usa %d de %d bytes (%s%%) -> %d bytes desperdiçados\n",
					p.PID, p.Name, pg.Index, pg.UsedBytes, PageSize,
					trimFloat(pct(pg.UsedBytes, PageSize)), PageSize-pg.UsedBytes)
			}
		}
	}
	if f.Underused == 0 {
		fmt.Println("Nenhuma página subutilizada.")
	}
	fmt.Printf("Total: %d página(s) subutilizada(s) de %d | %d bytes desperdiçados (%.1f%% do espaço alocado em páginas)\n",
		f.Underused, f.Pages, f.WastedByte, pct(f.WastedByte, f.Pages*PageSize))
}

func (m *Manager) PrintStats() {
	s := m.Stats
	fmt.Printf("\n--- ESTATÍSTICAS DE PAGINAÇÃO ---\n")
	fmt.Printf("Referências: %d | Hits: %d | Page Faults: %d | Taxa de falhas: %.1f%%\n",
		s.Refs, s.Hits, s.Faults, pct(s.Faults, s.Refs))
	fmt.Printf("Substituições: %d | Swap-outs (gravações): %d | Swap-ins (leituras): %d | Descartes limpos: %d | Cargas por demanda: %d\n",
		s.Replacements, s.SwapOuts, s.SwapIns, s.CleanDrops, s.DemandLoads)
}

func (m *Manager) PrintFragReport() {
	f := m.InternalFrag()
	ef := m.ExternalFrag()
	fmt.Printf("\n--- RELATÓRIO DE FRAGMENTAÇÃO ---\n")
	fmt.Printf("Interna (região paginada)  : %d página(s) subutilizada(s) de %d | %d bytes desperdiçados\n",
		f.Underused, f.Pages, f.WastedByte)
	ext := "NÃO"
	if ef.Present() {
		ext = "SIM"
	}
	fmt.Printf("Externa (área reservada)   : %s | %d lacuna(s) | livre: %s | maior bloco: %s | índice: %.1f%%\n",
		ext, ef.Holes, fmtKB(ef.TotalFree), fmtKB(ef.Largest), ef.Index())
}

// ---------- Ocupação ----------

func (m *Manager) PrintUsage() {
	ef := m.ExternalFrag()
	used := m.ReservedSize - ef.TotalFree
	critical := 0
	for _, p := range m.sortedProcs() {
		if p.Type == Critical {
			critical += p.Size
		}
	}
	framesUsed := 0
	for _, pg := range m.Frames {
		if pg != nil {
			framesUsed++
		}
	}
	total := m.totalPages()
	inSwap, untouched := 0, 0
	for _, p := range m.normalProcs() {
		for _, pg := range p.Pages {
			switch {
			case pg.Present:
			case pg.SwapSlot >= 0:
				inSwap++
			default:
				untouched++
			}
		}
	}
	slots := 0
	for _, u := range m.swapUsed {
		if u {
			slots++
		}
	}

	fmt.Printf("\n--- ANÁLISE DE OCUPAÇÃO DA MEMÓRIA ---\n")
	fmt.Printf("RAM Crítica/Reservada : %s %s / %s (%.1f%%) | núcleo base: %s | processos críticos: %s | livre: %s\n",
		bar(used, m.ReservedSize), fmtKB(used), fmtKB(m.ReservedSize), pct(used, m.ReservedSize),
		fmtKB(KernelBaseSize), fmtKB(critical), fmtKB(ef.TotalFree))
	fmt.Printf("RAM Dinâmica          : %s %d / %d molduras (%.1f%%) = %s de %s\n",
		bar(framesUsed, m.NumFrames), framesUsed, m.NumFrames, pct(framesUsed, m.NumFrames),
		fmtKB(framesUsed*PageSize), fmtKB(m.NumFrames*PageSize))
	fmt.Printf("Memória Virtual       : %s %d / %d páginas lógicas (%.1f%%) | na RAM: %d | no Swap: %d | ainda não carregadas: %d\n",
		bar(total, m.SwapSlots), total, m.SwapSlots, pct(total, m.SwapSlots), framesUsed, inSwap, untouched)
	fmt.Printf("Swap (disco)          : %s %d / %d slots em uso (%.1f%%)\n",
		bar(slots, m.SwapSlots), slots, m.SwapSlots, pct(slots, m.SwapSlots))
	fmt.Printf("Algoritmo de substituição: %s | Alocação contígua: %s | Processos ativos: %d\n",
		m.Algo.Label(), m.Fit.Label(), len(m.Procs))
}

func (m *Manager) PrintConfig() {
	fmt.Printf("\n--- CONFIGURAÇÃO DA MEMÓRIA ---\n")
	fmt.Printf("RAM física total          : %s = %s reservada (contígua) + %s dinâmica (paginada)\n",
		fmtKB(m.ReservedSize+m.NumFrames*PageSize), fmtKB(m.ReservedSize), fmtKB(m.NumFrames*PageSize))
	fmt.Printf("Página / Moldura          : %d KB | Molduras: %d\n", PageSize/KB, m.NumFrames)
	fmt.Printf("Swap                      : %d slots (%s)\n", m.SwapSlots, fmtKB(m.SwapSlots*PageSize))
	fmt.Printf("Substituição de páginas   : %s\n", m.Algo.Label())
	fmt.Printf("Alocação contígua         : %s\n", m.Fit.Label())
	if m.ReportEvery > 0 {
		fmt.Printf("Relatório periódico       : a cada %d operações de memória\n", m.ReportEvery)
	} else {
		fmt.Printf("Relatório periódico       : desligado\n")
	}
}

// ---------- Demo comparativo ----------

func Demo() {
	refs := []int{7, 0, 1, 2, 0, 3, 0, 4, 2, 3, 0, 3, 2, 1, 2, 0, 1, 7, 0, 1}
	const frames = 3

	fmt.Printf("\n[MEMÓRIA] Sequência de referências de páginas: %v\n", refs)
	fmt.Printf("Molduras na RAM: %d | Página lógica = Página do processo de teste (8 páginas)\n", frames)

	type result struct {
		algo   Algorithm
		faults int
	}
	var results []result
	for _, a := range AllAlgorithms() {
		sim := NewManager()
		sim.Quiet = true
		if err := sim.ResizeFrames(frames); err != nil {
			fmt.Println("demo:", err)
			return
		}
		sim.Algo = a
		p := sim.CreateNormal("demo", "sim", 8*PageSize)
		if p == nil {
			fmt.Println("demo: falha ao criar o processo de teste")
			return
		}

		fmt.Printf("\n=== %s ===\n", a.Label())
		fmt.Printf("%-4s %-14s %s\n", "Ref", "Molduras", "Resultado")
		for _, r := range refs {
			before := sim.Stats.Faults
			sim.Access(p, r, false)
			cells := make([]string, frames)
			for i := 0; i < frames; i++ {
				cells[i] = "-"
				if sim.Frames[i] != nil {
					cells[i] = strconv.Itoa(sim.Frames[i].Index)
				}
			}
			res := "hit"
			if sim.Stats.Faults > before {
				res = "PAGE FAULT"
			}
			fmt.Printf("%-4d [%-12s] %s\n", r, strings.Join(cells, " "), res)
		}
		fmt.Printf("Total: %d page faults em %d referências (%.1f%%)\n",
			sim.Stats.Faults, len(refs), pct(sim.Stats.Faults, len(refs)))
		results = append(results, result{a, sim.Stats.Faults})
	}

	fmt.Printf("\n--- COMPARAÇÃO ---\n")
	best := results[0]
	for _, r := range results {
		fmt.Printf("%-26s : %d page faults\n", r.algo.Label(), r.faults)
		if r.faults < best.faults {
			best = r
		}
	}
	fmt.Printf("Menos faltas de página nesta sequência: %s\n", best.algo.Label())
}
