package storage

import (
	"bytes"
	"errors"
	"fmt"
)

type RAIDLevel int

const (
	RAID0 RAIDLevel = 0
	RAID1 RAIDLevel = 1
	RAID4 RAIDLevel = 4
	RAID5 RAIDLevel = 5
)

type Disk struct {
	ID     int
	Failed bool
	Blocks [][]byte
}

type Cell struct {
	Disk    int
	Kind    string
	Logical int
}

type Array struct {
	Level         RAIDLevel
	Disks         []*Disk
	BlockSize     int
	Rows          int
	LogicalBlocks int

	PhysReads  int
	PhysWrites int
}

func NewArray(level RAIDLevel, numDisks, blockSize, logicalBlocks int) (*Array, error) {
	switch level {
	case RAID0, RAID1:
		if numDisks < 2 {
			return nil, fmt.Errorf("RAID %d exige pelo menos 2 discos", level)
		}
	case RAID4, RAID5:
		if numDisks < 3 {
			return nil, fmt.Errorf("RAID %d exige pelo menos 3 discos", level)
		}
	default:
		return nil, fmt.Errorf("nível RAID %d não suportado (use 0, 1, 4 ou 5)", level)
	}

	a := &Array{Level: level, BlockSize: blockSize, LogicalBlocks: logicalBlocks}
	dpr := a.dataPerRow(numDisks)
	a.Rows = (logicalBlocks + dpr - 1) / dpr
	for i := 0; i < numDisks; i++ {
		a.Disks = append(a.Disks, &Disk{ID: i, Blocks: make([][]byte, a.Rows)})
	}
	return a, nil
}

func (a *Array) dataPerRow(n int) int {
	switch a.Level {
	case RAID0:
		return n
	case RAID1:
		return 1
	default:
		return n - 1
	}
}

func (a *Array) Capacity() int { return a.LogicalBlocks }

func (a *Array) locate(L int) (disk, row, parity int) {
	n := len(a.Disks)
	switch a.Level {
	case RAID0:
		return L % n, L / n, -1
	case RAID1:
		return 0, L, -1
	case RAID4:
		return L % (n - 1), L / (n - 1), n - 1
	default:
		row = L / (n - 1)
		parity = (n - 1) - (row % n)
		disk = L % (n - 1)
		if disk >= parity {
			disk++
		}
		return disk, row, parity
	}
}

func (a *Array) Layout(row int) []Cell {
	n := len(a.Disks)
	cells := make([]Cell, n)
	switch a.Level {
	case RAID0:
		for d := 0; d < n; d++ {
			L := row*n + d
			cells[d] = Cell{Disk: d, Kind: "DADO", Logical: L}
			if L >= a.LogicalBlocks {
				cells[d] = Cell{Disk: d, Kind: "VAZIO", Logical: -1}
			}
		}
	case RAID1:
		for d := 0; d < n; d++ {
			cells[d] = Cell{Disk: d, Kind: "ESPELHO", Logical: row}
		}
	default:
		_, _, p := a.locate(row * (n - 1)) // paridade da faixa
		for d := 0; d < n; d++ {
			if d == p {
				cells[d] = Cell{Disk: d, Kind: "PARIDADE", Logical: -1}
				continue
			}
			slot := d
			if d > p {
				slot = d - 1
			}
			L := row*(n-1) + slot
			cells[d] = Cell{Disk: d, Kind: "DADO", Logical: L}
			if L >= a.LogicalBlocks {
				cells[d] = Cell{Disk: d, Kind: "VAZIO", Logical: -1}
			}
		}
	}
	return cells
}

func (a *Array) RowOf(L int) int {
	_, row, _ := a.locate(L)
	return row
}

func (a *Array) raw(d, row int) []byte {
	out := make([]byte, a.BlockSize)
	if b := a.Disks[d].Blocks[row]; b != nil {
		copy(out, b)
	}
	return out
}

func (a *Array) store(d, row int, data []byte) {
	buf := make([]byte, a.BlockSize)
	copy(buf, data)
	a.Disks[d].Blocks[row] = buf
	a.PhysWrites++
}

func (a *Array) load(d, row int) []byte {
	a.PhysReads++
	return a.raw(d, row)
}

func xorInto(dst, src []byte) {
	for i := range dst {
		dst[i] ^= src[i]
	}
}

var ErrDataLost = errors.New("dados irrecuperáveis (sem redundância suficiente)")

func (a *Array) Write(L int, data []byte) error {
	if L < 0 || L >= a.LogicalBlocks {
		return fmt.Errorf("bloco lógico %d fora do arranjo", L)
	}
	d, row, p := a.locate(L)

	switch a.Level {
	case RAID0:
		if a.Disks[d].Failed {
			return fmt.Errorf("Disco %d falhou: %w", d+1, ErrDataLost)
		}
		a.store(d, row, data)

	case RAID1:
		ok := false
		for _, disk := range a.Disks {
			if !disk.Failed {
				a.store(disk.ID, row, data)
				ok = true
			}
		}
		if !ok {
			return errors.New("todos os discos do espelho falharam")
		}

	default:
		newData := make([]byte, a.BlockSize)
		copy(newData, data)

		var old []byte
		if a.Disks[d].Failed {
			rec, _, err := a.Reconstruct(d, row)
			if err != nil {
				return err
			}
			old = rec
		} else {
			old = a.load(d, row)
		}

		if !a.Disks[p].Failed {
			parity := a.load(p, row)
			xorInto(parity, old)
			xorInto(parity, newData)
			a.store(p, row, parity)
		}
		if !a.Disks[d].Failed {
			a.store(d, row, newData)
		}
	}
	return nil
}

func (a *Array) Read(L int) ([]byte, error) {
	if L < 0 || L >= a.LogicalBlocks {
		return nil, fmt.Errorf("bloco lógico %d fora do arranjo", L)
	}
	d, row, _ := a.locate(L)

	switch a.Level {
	case RAID0:
		if a.Disks[d].Failed {
			return nil, fmt.Errorf("Disco %d falhou: %w", d+1, ErrDataLost)
		}
		return a.load(d, row), nil
	case RAID1:
		for _, disk := range a.Disks {
			if !disk.Failed {
				return a.load(disk.ID, row), nil
			}
		}
		return nil, errors.New("todos os discos do espelho falharam")
	default:
		if a.Disks[d].Failed {
			rec, _, err := a.Reconstruct(d, row)
			return rec, err
		}
		return a.load(d, row), nil
	}
}

func (a *Array) Reconstruct(d, row int) ([]byte, []int, error) {
	switch a.Level {
	case RAID0:
		return nil, nil, ErrDataLost
	case RAID1:
		for _, disk := range a.Disks {
			if disk.ID != d && !disk.Failed {
				return a.load(disk.ID, row), []int{disk.ID}, nil
			}
		}
		return nil, nil, ErrDataLost
	default:
		out := make([]byte, a.BlockSize)
		used := []int{}
		for _, disk := range a.Disks {
			if disk.ID == d {
				continue
			}
			if disk.Failed {
				return nil, nil, fmt.Errorf("duas falhas na faixa %d: %w", row, ErrDataLost)
			}
			xorInto(out, a.load(disk.ID, row))
			used = append(used, disk.ID)
		}
		return out, used, nil
	}
}

func (a *Array) Fail(d int) error {
	if d < 0 || d >= len(a.Disks) {
		return fmt.Errorf("disco %d inexistente", d+1)
	}
	a.Disks[d].Failed = true
	return nil
}

func (a *Array) Rebuild(d int) error {
	if d < 0 || d >= len(a.Disks) {
		return fmt.Errorf("disco %d inexistente", d+1)
	}
	if !a.Disks[d].Failed {
		return fmt.Errorf("Disco %d não está falho", d+1)
	}
	if a.Level == RAID0 {
		return fmt.Errorf("RAID 0 não tem redundância: %w", ErrDataLost)
	}
	fresh := make([][]byte, a.Rows)
	for row := 0; row < a.Rows; row++ {
		blk, _, err := a.Reconstruct(d, row)
		if err != nil {
			return err
		}
		fresh[row] = blk
		a.PhysWrites++
	}
	a.Disks[d].Blocks = fresh
	a.Disks[d].Failed = false
	return nil
}

func (a *Array) snapshot(d int) [][]byte {
	out := make([][]byte, a.Rows)
	for i, b := range a.Disks[d].Blocks {
		if b != nil {
			out[i] = append([]byte(nil), b...)
		}
	}
	return out
}

func sameBlock(x, y []byte, size int) bool {
	bx, by := make([]byte, size), make([]byte, size)
	copy(bx, x)
	copy(by, y)
	return bytes.Equal(bx, by)
}

func (a *Array) CapacityInfo() (usable int, tolerance string) {
	n := len(a.Disks)
	switch a.Level {
	case RAID0:
		return n * a.Rows, "nenhuma"
	case RAID1:
		return a.Rows, fmt.Sprintf("até %d discos", n-1)
	default:
		return (n - 1) * a.Rows, "1 disco"
	}
}
