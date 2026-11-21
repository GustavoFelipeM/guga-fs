package memory

import "fmt"

func (m *Manager) CreateNormal(name, owner string, size int) *Process {
	if m.nameTaken(name) {
		m.pf("create_process: já existe um processo chamado '%s'.\n", name)
		return nil
	}
	n := (size + PageSize - 1) / PageSize
	if m.totalPages()+n > m.SwapSlots {
		m.pf("\n[FALHA] Memória virtual insuficiente: o processo precisa de %d página(s), mas só restam %d de %d no espaço virtual (swap).\n",
			n, m.SwapSlots-m.totalPages(), m.SwapSlots)
		return nil
	}

	p := &Process{PID: m.nextPID, Name: name, Type: Normal, Owner: owner, Size: size}
	m.nextPID++
	for i := 0; i < n; i++ {
		used := PageSize
		if i == n-1 {
			used = size - (n-1)*PageSize
		}
		p.Pages = append(p.Pages, &Page{Index: i, Proc: p, UsedBytes: used, Frame: -1, SwapSlot: -1})
	}
	m.Procs[p.PID] = p

	m.pf("\n[CRIAÇÃO DE PROCESSO NÃO CRÍTICO - MEMÓRIA VIRTUAL] PID %d | dono: %s\n", p.PID, owner)
	m.pf("Tamanho do Processo : %s (%d Bytes)\n", fmtKB(size), size)
	m.pf("Tamanho da Página   : %d KB (%d Bytes)\n", PageSize/KB, PageSize)
	m.pf("Cálculo de Páginas  : %s / %d KB = %s -> Requer %d Página(s) Lógica(s) (%s).\n",
		fmtKB(size), PageSize/KB, trimFloat(float64(size)/PageSize), n, pagesText(n))

	m.pf("\n--- ANÁLISE DE FRAGMENTAÇÃO INTERNA ---\n")
	if n <= 6 {
		for _, pg := range p.Pages {
			m.pf("Página Lógica %d : %d Bytes ocupados (%s%%)\n", pg.Index, pg.UsedBytes, trimFloat(float64(pg.UsedBytes)*100/PageSize))
		}
	} else {
		m.pf("Páginas Lógicas 0 a %d : %d Bytes ocupados em cada uma (100%%)\n", n-2, PageSize)
		last := p.Pages[n-1]
		m.pf("Página Lógica %d : %d Bytes ocupados (%s%%)\n", last.Index, last.UsedBytes, trimFloat(float64(last.UsedBytes)*100/PageSize))
	}
	waste := n*PageSize - size
	if waste > 0 {
		m.pf("Fragmentação Interna Total: %d Bytes de desperdício na Página Lógica %d.\n", waste, n-1)
	} else {
		m.pf("Fragmentação Interna Total: 0 Bytes (o processo preenche todas as páginas).\n")
	}
	m.pf("Status: Tabela de Páginas gerada sem alocar molduras na RAM (Alocação por demanda).\n")
	m.afterOp()
	return p
}

func (m *Manager) Terminate(p *Process) {
	if p.Type == Critical {
		m.terminateCritical(p)
	} else {
		var frames []int
		slots := 0
		for _, pg := range p.Pages {
			if pg.Present {
				frames = append(frames, pg.Frame)
				m.Frames[pg.Frame] = nil
			}
			if pg.SwapSlot >= 0 {
				m.swapUsed[pg.SwapSlot] = false
				slots++
			}
		}
		m.pf("\n[ENCERRAMENTO DE PROCESSO] '%s' (PID %d) finalizado.\n", p.Name, p.PID)
		m.pf("Molduras liberadas na RAM dinâmica: %s | Slots de swap liberados: %d\n", rangesText(frames), slots)
	}
	delete(m.Procs, p.PID)
	m.afterOp()
}

// ---------- Tradução de endereços ----------

func (m *Manager) Translate(p *Process, pageIdx, offset int) {
	m.pf("\n[TRADUÇÃO DE ENDEREÇO: PÁGINA LÓGICA -> MOLDURA FÍSICA]\n")

	if p.Type == Critical {
		addr := pageIdx*PageSize + offset
		if addr < 0 || addr >= p.Size {
			m.pf("[VIOLAÇÃO DE ENDEREÇO] Endereço lógico %d fora do espaço do processo crítico '%s' (0..%d).\n", addr, p.Name, p.Size-1)
			return
		}
		m.pf("Processo crítico '%s': alocação contígua, sem paginação.\n", p.Name)
		m.pf("Endereço lógico %d -> Endereço físico 0x%X (base %s + deslocamento %d) [ÁREA RESERVADA]\n",
			addr, p.Base+addr, fmtMB(p.Base), addr)
		return
	}

	if pageIdx < 0 || pageIdx >= len(p.Pages) {
		m.pf("[VIOLAÇÃO DE ENDEREÇO] Página Lógica %d inexistente em '%s' (válidas: 0..%d).\n", pageIdx, p.Name, len(p.Pages)-1)
		return
	}
	pg := p.Pages[pageIdx]
	if offset < 0 || offset >= pg.UsedBytes {
		m.pf("[VIOLAÇÃO DE ENDEREÇO] Deslocamento %d fora da Página Lógica %d (usa %d bytes).\n", offset, pageIdx, pg.UsedBytes)
		return
	}

	if pg.Present {
		phys := m.DynamicBase() + pg.Frame*PageSize + offset
		m.pf("Solicitação: Página Lógica %d -> Mapeada na Moldura Física %d (RAM Dinâmica) [HIT]\n", pageIdx, pg.Frame)
		m.pf("Endereço lógico %d -> Endereço físico 0x%X (base dinâmica %s + moldura %d x %d + deslocamento %d)\n",
			pageIdx*PageSize+offset, phys, fmtMB(m.DynamicBase()), pg.Frame, PageSize, offset)
		return
	}
	m.pf("Solicitação: Página Lógica %d -> NÃO MAPEADA (bit de presença = 0) [MISS - causaria PAGE FAULT]\n", pageIdx)
	if pg.SwapSlot >= 0 {
		m.pf("A página está no Swap (slot %d). Use 'execute_step' para efetivar o acesso e trazê-la para a RAM.\n", pg.SwapSlot)
	} else {
		m.pf("A página ainda não foi carregada (paginação por demanda). Use 'execute_step' para efetivar o acesso.\n")
	}
}

// ---------- Execução de uma referência à memória ----------

func (m *Manager) Access(p *Process, pageIdx int, write bool) bool {
	mode := "leitura"
	if write {
		mode = "escrita"
	}

	if p.Type == Critical {
		m.pf("\n[CPU - REFERÊNCIA DE MEMÓRIA EM EXECUÇÃO]\n")
		m.pf("%s (processo crítico) acessa a Área Reservada [%s até %s] diretamente: sem tabela de páginas e sem page fault.\n",
			p.Name, fmtMB(p.Base), fmtMB(p.Base+p.Size))
		return true
	}

	if pageIdx < 0 || pageIdx >= len(p.Pages) {
		m.pf("\n[VIOLAÇÃO DE ENDEREÇO] Página Lógica %d inexistente em '%s' (válidas: 0..%d). Acesso abortado.\n", pageIdx, p.Name, len(p.Pages)-1)
		return false
	}

	m.Clock++
	m.Stats.Refs++
	pg := p.Pages[pageIdx]

	m.pf("\n[CPU - REFERÊNCIA DE MEMÓRIA EM EXECUÇÃO]\n")
	m.pf("%s requisita acesso à Página Lógica %d (%s)...\n", p.Name, pageIdx, mode)

	if pg.Present {
		m.Stats.Hits++
		pg.LastUsed = m.Clock
		pg.Referenced = true
		if write {
			pg.Dirty = true
		}
		m.pf("- Consulta Tabela de Páginas -> Página Lógica %d presente na Moldura Física %d (RAM Dinâmica) [HIT]\n", pageIdx, pg.Frame)
		m.afterOp()
		return true
	}

	m.Stats.Faults++
	m.pf("- Consulta Tabela de Páginas -> Página Lógica %d NÃO ESTÁ NA RAM!\n", pageIdx)
	m.pf("\n[PAGE FAULT DETECTADO]: Página Lógica %d ausente na RAM!\n", pageIdx)

	frame := m.freeFrame()
	if frame < 0 {
		m.Stats.Replacements++
		m.pf("Memória RAM cheia. Executando Substituição de Página (Algoritmo %s):\n", m.Algo.Short())
		frame = m.pickVictim()
		m.evict(frame)
	} else {
		m.pf("Há moldura livre na RAM dinâmica (Moldura Física %d): nenhuma substituição necessária.\n", frame)
	}
	m.load(pg, frame, write)
	m.afterOp()
	return true
}

func (m *Manager) freeFrame() int {
	for i, pg := range m.Frames {
		if pg == nil {
			return i
		}
	}
	return -1
}

func (m *Manager) pickVictim() int {
	switch m.Algo {
	case FIFO:
		best := 0
		for i, pg := range m.Frames {
			if pg.LoadedAt < m.Frames[best].LoadedAt {
				best = i
			}
		}
		v := m.Frames[best]
		m.pf("- Analisando ordem de chegada: Página Lógica %d do PID %d (%s) está na RAM há %d ms (Mais antiga).\n",
			v.Index, v.Proc.PID, v.Proc.Name, (m.Clock-v.LoadedAt)*MsPerTick)
		return best

	case SecondChance:
		for {
			i := m.hand
			v := m.Frames[i]
			m.hand = (m.hand + 1) % m.NumFrames
			if v.Referenced {
				v.Referenced = false
				m.pf("- Ponteiro na Moldura %d (Página Lógica %d do PID %d): bit R=1 -> Segunda Chance (R := 0), ponteiro avança.\n",
					i, v.Index, v.Proc.PID)
				continue
			}
			m.pf("- Ponteiro na Moldura %d (Página Lógica %d do PID %d): bit R=0 -> escolhida como vítima.\n",
				i, v.Index, v.Proc.PID)
			return i
		}

	default:
		best := 0
		for i, pg := range m.Frames {
			if pg.LastUsed < m.Frames[best].LastUsed {
				best = i
			}
		}
		v := m.Frames[best]
		m.pf("- Analisando histórico de acesso: Página Lógica %d do PID %d (%s) não é usada há %d ms (Mais antiga).\n",
			v.Index, v.Proc.PID, v.Proc.Name, (m.Clock-v.LastUsed)*MsPerTick)
		return best
	}
}

func (m *Manager) allocSwap() int {
	for i, used := range m.swapUsed {
		if !used {
			m.swapUsed[i] = true
			return i
		}
	}
	return -1
}

func (m *Manager) swapIO(write bool, slot int) string {
	if m.SwapIO == nil {
		return ""
	}
	track := m.SwapIO(write, slot)
	kind := "leitura"
	if write {
		kind = "escrita"
	}
	return fmt.Sprintf(" [E/S: %s do slot %d enfileirada na trilha %d do disco]", kind, slot, track)
}

func (m *Manager) evict(frame int) {
	v := m.Frames[frame]
	if v.Dirty || v.SwapSlot < 0 {
		if v.SwapSlot < 0 {
			v.SwapSlot = m.allocSwap()
		}
		if v.SwapSlot < 0 {
			m.pf("- ERRO: swap cheio; conteúdo da página descartado.\n")
		} else {
			m.Stats.SwapOuts++
			m.pf("- Removendo Página Lógica %d do PID %d da RAM -> Salvando no Swap (Disco), slot %d.%s\n",
				v.Index, v.Proc.PID, v.SwapSlot, m.swapIO(true, v.SwapSlot))
		}
	} else {
		m.Stats.CleanDrops++
		m.pf("- Removendo Página Lógica %d do PID %d da RAM -> página limpa, cópia válida no Swap (slot %d): descartada sem escrita em disco.\n",
			v.Index, v.Proc.PID, v.SwapSlot)
	}
	v.Present = false
	v.Frame = -1
	v.Referenced = false
	v.Dirty = false
	m.Frames[frame] = nil
}

func (m *Manager) load(pg *Page, frame int, write bool) {
	if pg.SwapSlot >= 0 {
		m.Stats.SwapIns++
		m.pf("- Origem: Swap (Disco), slot %d.%s\n", pg.SwapSlot, m.swapIO(false, pg.SwapSlot))
	} else {
		m.Stats.DemandLoads++
		m.pf("- Origem: primeira referência (paginação por demanda), sem leitura de swap.\n")
	}
	m.pf("- Carregando Página Lógica %d do %s na Moldura Física %d (RAM Dinâmica).\n", pg.Index, pg.Proc.Name, frame)

	m.Frames[frame] = pg
	pg.Present = true
	pg.Frame = frame
	pg.LoadedAt = m.Clock
	pg.LastUsed = m.Clock
	pg.Referenced = true
	pg.Dirty = write
	m.pf("- Tabela de Páginas atualizada: Página Lógica %d ---> Moldura Física %d.\n", pg.Index, frame)
}
