package memory

import "fmt"

type ExtFrag struct {
	TotalFree int
	Largest   int
	Holes     int
}

func (m *Manager) ExternalFrag() ExtFrag {
	var e ExtFrag
	for _, b := range m.Blocks {
		if !b.Free() {
			continue
		}
		e.Holes++
		e.TotalFree += b.Size
		if b.Size > e.Largest {
			e.Largest = b.Size
		}
	}
	return e
}

func (e ExtFrag) Present() bool { return e.Holes > 1 }

func (e ExtFrag) Index() float64 {
	if e.TotalFree == 0 {
		return 0
	}
	return (1 - float64(e.Largest)/float64(e.TotalFree)) * 100
}

func (m *Manager) findHole(size int) int {
	idx := -1
	for i, b := range m.Blocks {
		if !b.Free() || b.Size < size {
			continue
		}
		switch m.Fit {
		case BestFit:
			if idx < 0 || b.Size < m.Blocks[idx].Size {
				idx = i
			}
		case WorstFit:
			if idx < 0 || b.Size > m.Blocks[idx].Size {
				idx = i
			}
		default:
			return i
		}
	}
	return idx
}

func (m *Manager) CreateCritical(name, owner string, size int) *Process {
	if m.nameTaken(name) {
		m.pf("create_process: já existe um processo chamado '%s'.\n", name)
		return nil
	}
	idx := m.findHole(size)
	if idx < 0 {
		ef := m.ExternalFrag()
		m.pf("\n[FALHA DE ALOCAÇÃO CONTÍGUA] '%s' requer %s contíguos na Área Reservada.\n", name, fmtKB(size))
		m.pf("Memória livre total: %s | Maior bloco contíguo: %s\n", fmtKB(ef.TotalFree), fmtKB(ef.Largest))
		if ef.TotalFree >= size {
			m.pf("Causa: FRAGMENTAÇÃO EXTERNA - há espaço suficiente, porém dividido em %d fragmentos não contíguos.\n", ef.Holes)
			m.pf("Solução: execute 'memory_compact --reserved' e tente novamente.\n")
		} else {
			m.pf("Causa: Área Reservada insuficiente (mesmo compactada, faltam %s).\n", fmtKB(size-ef.TotalFree))
		}
		return nil
	}

	hole := m.Blocks[idx]
	p := &Process{PID: m.nextPID, Name: name, Type: Critical, Owner: owner, Size: size, Base: hole.Start}
	m.nextPID++
	m.Procs[p.PID] = p

	if hole.Size > size {
		rest := Block{Start: hole.Start + size, Size: hole.Size - size, PID: -1, Label: "livre"}
		tail := append([]Block{rest}, m.Blocks[idx+1:]...)
		m.Blocks = append(m.Blocks[:idx+1], tail...)
	}
	m.Blocks[idx] = Block{Start: hole.Start, Size: size, PID: p.PID, Label: name}

	m.pf("\n[CRIAÇÃO DE PROCESSO CRÍTICO - ÁREA RESERVADA] PID %d | dono: %s | política: %s\n", p.PID, owner, m.Fit.Label())
	m.pf("- Alocado de forma contígua na Área Reservada: [%s até %s] (%s)\n", fmtMB(p.Base), fmtMB(p.Base+size), fmtKB(size))
	m.afterOp()
	return p
}

func (m *Manager) coalesce() {
	out := make([]Block, 0, len(m.Blocks))
	for _, b := range m.Blocks {
		if n := len(out); n > 0 && out[n-1].Free() && b.Free() {
			out[n-1].Size += b.Size
			continue
		}
		out = append(out, b)
	}
	m.Blocks = out
}

func (m *Manager) terminateCritical(p *Process) {
	for i, b := range m.Blocks {
		if b.PID != p.PID {
			continue
		}
		m.pf("\n[ENCERRAMENTO DE PROCESSO CRÍTICO] Processo '%s' finalizado.\n", p.Name)
		m.pf("Área de %s no intervalo [%s a %s] liberada na Área Reservada da RAM.\n", fmtKB(b.Size), fmtMB(b.Start), fmtMB(b.End()))
		m.Blocks[i] = Block{Start: b.Start, Size: b.Size, PID: -1, Label: "livre"}
		break
	}
	m.coalesce()
}

func (m *Manager) Compact() bool {
	ef := m.ExternalFrag()
	if ef.Holes == 0 || (ef.Holes == 1 && m.Blocks[len(m.Blocks)-1].Free()) {
		m.pf("\n[COMPACTAÇÃO DA ÁREA RESERVADA] Nada a fazer: o espaço livre já é um único bloco contíguo (ou não há espaço livre).\n")
		return false
	}

	m.pf("\n[EXECUTANDO COMPACTAÇÃO DA ÁREA RESERVADA DA RAM]\n")
	m.pf("Realocando blocos contíguos para eliminar a fragmentação externa...\n")

	cursor, moved := 0, 0
	out := make([]Block, 0, len(m.Blocks))
	for _, b := range m.Blocks {
		if b.Free() {
			continue
		}
		if b.Start != cursor {
			m.pf("- Mapeamento de '%s': [%s a %s] -> Ajustado para: [%s a %s]\n",
				b.Label, fmtMB(b.Start), fmtMB(b.End()), fmtMB(cursor), fmtMB(cursor+b.Size))
			moved += b.Size
			b.Start = cursor
			if p, ok := m.Procs[b.PID]; ok {
				p.Base = cursor
			}
		}
		cursor += b.Size
		out = append(out, b)
	}
	if cursor < m.ReservedSize {
		out = append(out, Block{Start: cursor, Size: m.ReservedSize - cursor, PID: -1, Label: "livre"})
	}
	m.Blocks = out
	m.Stats.Compactions++

	m.pf("\n--- MAPA APÓS COMPACTAÇÃO ---\n")
	if !m.Quiet {
		m.printBlocks()
	}
	after := m.ExternalFrag()
	m.pf("Resultado: Fragmentação externa eliminada. Maior bloco contíguo: %s.\n", fmtKB(after.Largest))
	m.pf("Custo da compactação: %s de dados copiados na RAM.\n", fmtKB(moved))
	m.afterOp()
	return true
}

func (m *Manager) printBlocks() {
	holes := m.ExternalFrag().Holes
	n := 0
	for _, b := range m.Blocks {
		rng := fmt.Sprintf("[%s - %s]", fmtMB(b.Start), fmtMB(b.End()))
		label, state := b.Label, "OCUPADO"
		if b.Free() {
			n++
			if holes > 1 {
				label, state = fmt.Sprintf("FRAGMENTO LIVRE %d", n), "LIVRE - LACUNA"
			} else {
				label, state = "Bloco Livre Contíguo", "LIVRE"
			}
		}
		fmt.Printf("%-20s : %-20s %-12s [%s]\n", rng, label, "("+fmtKB(b.Size)+")", state)
	}
}
