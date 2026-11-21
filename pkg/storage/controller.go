package storage

import (
	"fmt"
	"sort"
	"strings"
)

type Request struct {
	Track int
	Write bool
	Block int
}

type Controller struct {
	Array       *Array
	Algo        Algorithm
	Head        int
	Tracks      int
	Up          bool
	TotalBlocks int
	BlockSize   int

	Pending     []Request
	Background  []int
	TotalMoved  int
	labels      map[int]string
	touchedRows map[int]bool
	wroteInOp   bool
}

func NewController(blockSize, totalBlocks, tracks int) (*Controller, error) {
	arr, err := NewArray(RAID5, 3, blockSize, totalBlocks)
	if err != nil {
		return nil, err
	}
	return &Controller{
		Array:       arr,
		Algo:        SSTF,
		Tracks:      tracks,
		Up:          true,
		TotalBlocks: totalBlocks,
		BlockSize:   blockSize,
		labels:      map[int]string{},
		touchedRows: map[int]bool{},
	}, nil
}

func (c *Controller) TrackOf(block int) int {
	t := block * c.Tracks / c.TotalBlocks
	if t >= c.Tracks {
		t = c.Tracks - 1
	}
	return t
}

func (c *Controller) WriteBlock(block int, data []byte, label string) {
	c.Pending = append(c.Pending, Request{Track: c.TrackOf(block), Write: true, Block: block})
	c.labels[block] = label
	if err := c.Array.Write(block, data); err != nil {
		fmt.Printf("[E/S] ERRO ao gravar bloco %d: %v\n", block, err)
		return
	}
	c.touchedRows[c.Array.RowOf(block)] = true
	c.wroteInOp = true
}

func (c *Controller) ReadBlock(block int, label string) []byte {
	c.Pending = append(c.Pending, Request{Track: c.TrackOf(block), Write: false, Block: block})
	data, err := c.Array.Read(block)
	if err != nil {
		fmt.Printf("[E/S] ERRO ao ler bloco %d: %v\n", block, err)
		return make([]byte, c.BlockSize)
	}
	return data
}

func (c *Controller) FreeBlock(block int) {
	delete(c.labels, block)
}

func (c *Controller) Flush(operation string) {
	if len(c.Pending) == 0 && len(c.Background) == 0 {
		return
	}

	tracks := append([]int(nil), c.Background...)
	for _, r := range c.Pending {
		tracks = append(tracks, r.Track)
	}

	fmt.Printf("\n[E/S] %s\n", operation)
	fmt.Printf("[E/S] Requisições de leitura/escrita pendentes nas trilhas: %v\n", tracks)
	fmt.Printf("Posição atual da cabeça de leitura: Trilha %d\n", c.Head)

	res := Schedule(c.Algo, c.Head, tracks, c.Tracks-1, c.Up)
	res.Print()
	c.Head = res.FinalHead()
	c.TotalMoved += res.Total

	c.Pending = nil
	c.Background = nil

	if c.wroteInOp {
		c.printDistribution()
	}
	c.wroteInOp = false
	c.touchedRows = map[int]bool{}
}

func raidName(l RAIDLevel) string { return fmt.Sprintf("RAID %d", int(l)) }

func (c *Controller) label(L int) string {
	if s, ok := c.labels[L]; ok {
		return s
	}
	return "livre"
}

func (c *Controller) blockKB() string {
	if c.BlockSize%1024 == 0 {
		return fmt.Sprintf("%d KB", c.BlockSize/1024)
	}
	return fmt.Sprintf("%d B", c.BlockSize)
}

func (c *Controller) cellText(cell Cell, row int) string {
	switch cell.Kind {
	case "DADO":
		if _, ok := c.labels[cell.Logical]; !ok {
			return "[vazio]"
		}
		return fmt.Sprintf("[Bloco %02d: %s (%s)]", cell.Logical, c.label(cell.Logical), c.blockKB())
	case "ESPELHO":
		return fmt.Sprintf("[ESPELHO : cópia do Bloco %02d]", cell.Logical)
	case "PARIDADE":
		srcs := []string{}
		for _, o := range c.Array.Layout(row) {
			if o.Kind == "DADO" {
				srcs = append(srcs, fmt.Sprintf("Bloco %02d", o.Logical))
			}
		}
		return fmt.Sprintf("[PARIDADE : XOR(%s)]", strings.Join(srcs, ", "))
	}
	return "[vazio]"
}

func (c *Controller) printDistribution() {
	a := c.Array
	fmt.Printf("\nArranjo Ativo: %s (%d Discos Físicos, Blocos de %s)\n", raidName(a.Level), len(a.Disks), c.blockKB())
	fmt.Println("[E/S - DISTRIBUIÇÃO EM DISCO]")

	rows := []int{}
	for r := range c.touchedRows {
		rows = append(rows, r)
	}
	sort.Ints(rows)

	for _, r := range rows {
		fmt.Printf("Faixa %d:\n", r)
		for _, cell := range a.Layout(r) {
			state := ""
			if a.Disks[cell.Disk].Failed {
				state = " (FALHO)"
			}
			fmt.Printf("  Disco %d: %s%s\n", cell.Disk+1, c.cellText(cell, r), state)
		}
	}
}

func (c *Controller) Status() {
	a := c.Array
	usable, tol := a.CapacityInfo()
	fmt.Printf("\n[ARRANJO] %s | %d discos | bloco de %s | %d blocos por disco\n",
		raidName(a.Level), len(a.Disks), c.blockKB(), a.Rows)
	fmt.Printf("Capacidade útil: %d blocos (%d KB) | Tolerância a falhas: %s\n",
		usable, usable*c.BlockSize/1024, tol)
	for _, d := range a.Disks {
		st := "OK"
		if d.Failed {
			st = "FALHO"
		}
		used := 0
		for _, b := range d.Blocks {
			if b != nil {
				used++
			}
		}
		fmt.Printf("  Disco %d: %-5s | blocos gravados: %d/%d\n", d.ID+1, st, used, a.Rows)
	}
	fmt.Printf("E/S física acumulada: %d leituras, %d escritas\n", a.PhysReads, a.PhysWrites)
	fmt.Printf("Escalonador: %s | cabeça na trilha %d | deslocamento acumulado: %d trilhas\n",
		c.Algo, c.Head, c.TotalMoved)
}

func (c *Controller) Reconfigure(level RAIDLevel, disks int) error {
	fresh, err := NewArray(level, disks, c.BlockSize, c.TotalBlocks)
	if err != nil {
		return err
	}
	for L := range c.labels {
		data, err := c.Array.Read(L)
		if err != nil {
			return fmt.Errorf("não foi possível migrar o bloco %d: %w", L, err)
		}
		if err := fresh.Write(L, data); err != nil {
			return err
		}
	}
	fresh.PhysReads, fresh.PhysWrites = 0, 0
	c.Array = fresh
	return nil
}

func (c *Controller) FailureTest(d int) {
	a := c.Array
	if d < 0 || d >= len(a.Disks) {
		fmt.Printf("raid: disco %d inexistente (use 1..%d)\n", d+1, len(a.Disks))
		return
	}
	if a.Disks[d].Failed {
		fmt.Printf("raid: Disco %d já está falho.\n", d+1)
		return
	}

	fmt.Println("\n[TESTE DE FALHA SIMULADO]")
	original := a.snapshot(d)
	a.Fail(d)
	fmt.Printf("* Falha forçada no Disco %d!\n", d+1)

	tested, lost := 0, 0
	for row := 0; row < a.Rows; row++ {
		cell := a.Layout(row)[d]
		if !c.rowInUse(row) || cell.Kind == "VAZIO" {
			continue
		}
		tested++

		what := fmt.Sprintf("PARIDADE da faixa %d", row)
		if cell.Kind != "PARIDADE" {
			what = fmt.Sprintf("Bloco %02d", cell.Logical)
		}

		rec, used, err := a.Reconstruct(d, row)
		if err != nil {
			lost++
			fmt.Printf("* %s: SEM REDUNDÂNCIA -> dados perdidos [FALHA]\n", what)
			continue
		}
		ok := sameBlock(rec, original[row], a.BlockSize)
		status := "[OK]"
		if !ok {
			status = "[ERRO]"
		}
		if a.Level == RAID1 {
			fmt.Printf("* Recuperando %s copiando do Disco %d -> Reconstrução %s\n", what, used[0]+1, status)
		} else {
			names := make([]string, len(used))
			for i, u := range used {
				names[i] = fmt.Sprintf("Disco %d", u+1)
			}
			fmt.Printf("* Recuperando %s via operação XOR: (%s) -> Reconstrução %s\n",
				what, strings.Join(names, " XOR "), status)
		}
	}
	if tested == 0 {
		fmt.Println("* Nenhum bloco em uso neste disco; nada a reconstruir.")
	}

	if a.Level == RAID0 {
		a.Disks[d].Blocks = original
		a.Disks[d].Failed = false
		fmt.Printf("* Disco %d restaurado do backup do teste (RAID 0 não consegue se reconstruir sozinho).\n", d+1)
	} else if err := a.Rebuild(d); err != nil {
		fmt.Printf("* Rebuild do Disco %d falhou: %v\n", d+1, err)
	} else {
		fmt.Printf("* Disco %d substituído e reconstruído por completo [OK]\n", d+1)
	}
	if lost > 0 {
		fmt.Printf("Resultado: %d bloco(s) em uso, %d perdido(s).\n", tested, lost)
	}
}

func (c *Controller) rowInUse(row int) bool {
	for _, cell := range c.Array.Layout(row) {
		if cell.Kind == "DADO" || cell.Kind == "ESPELHO" {
			if _, ok := c.labels[cell.Logical]; ok {
				return true
			}
		}
	}
	return false
}

func (c *Controller) validBlock(block int) error {
	if block < 0 || block >= c.Array.LogicalBlocks {
		return fmt.Errorf("bloco lógico %d fora do arranjo", block)
	}
	return nil
}

func (c *Controller) PeekBlock(block int) ([]byte, error) {
	if err := c.validBlock(block); err != nil {
		return nil, err
	}
	return c.Array.Read(block)
}

func (c *Controller) PokeBlock(block int, data []byte) error {
	if err := c.validBlock(block); err != nil {
		return err
	}
	return c.Array.Write(block, data)
}

func (c *Controller) RawWriteBlock(block int, data []byte) error {
	if err := c.validBlock(block); err != nil {
		return err
	}
	d, row, _ := c.Array.locate(block)
	if c.Array.Disks[d].Failed {
		return fmt.Errorf("Disco %d está falho", d+1)
	}
	buf := make([]byte, c.BlockSize)
	copy(buf, data)
	c.Array.Disks[d].Blocks[row] = buf
	return nil
}

func (c *Controller) RepairBlock(block int) ([]byte, error) {
	if err := c.validBlock(block); err != nil {
		return nil, err
	}
	d, row, _ := c.Array.locate(block)
	rec, _, err := c.Array.Reconstruct(d, row)
	return rec, err
}

func (c *Controller) RedundancyName() string { return raidName(c.Array.Level) }
