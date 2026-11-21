package memory

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	KB       = 1024
	MB       = 1024 * KB
	PageSize = 4 * KB

	MsPerTick = 5

	DefaultReservedSize = 4 * MB
	KernelBaseSize      = 1 * MB
	DefaultFrames       = 64
	DefaultSwapSlots    = 256
	FirstUserPID        = 100
)

// ---------- Tipos básicos ----------

type ProcType int

const (
	Normal ProcType = iota
	Critical
)

func (t ProcType) String() string {
	if t == Critical {
		return "critical"
	}
	return "normal"
}

type Algorithm string

const (
	LRU          Algorithm = "lru"
	SecondChance Algorithm = "second_chance"
	FIFO         Algorithm = "fifo"
)

func ParseAlgorithm(s string) (Algorithm, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "lru":
		return LRU, true
	case "second_chance", "segunda_chance", "secondchance", "clock", "relogio", "relógio":
		return SecondChance, true
	case "fifo":
		return FIFO, true
	}
	return "", false
}

func AllAlgorithms() []Algorithm { return []Algorithm{FIFO, LRU, SecondChance} }

func (a Algorithm) Label() string {
	switch a {
	case LRU:
		return "LRU (Least Recently Used)"
	case SecondChance:
		return "Segunda Chance (Clock)"
	case FIFO:
		return "FIFO"
	}
	return string(a)
}

func (a Algorithm) Short() string {
	switch a {
	case LRU:
		return "LRU"
	case SecondChance:
		return "Segunda Chance"
	case FIFO:
		return "FIFO"
	}
	return string(a)
}

type FitPolicy string

const (
	FirstFit FitPolicy = "first"
	BestFit  FitPolicy = "best"
	WorstFit FitPolicy = "worst"
)

func ParseFit(s string) (FitPolicy, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "first", "first_fit", "firstfit":
		return FirstFit, true
	case "best", "best_fit", "bestfit":
		return BestFit, true
	case "worst", "worst_fit", "worstfit":
		return WorstFit, true
	}
	return "", false
}

func (f FitPolicy) Label() string {
	switch f {
	case BestFit:
		return "Best Fit"
	case WorstFit:
		return "Worst Fit"
	}
	return "First Fit"
}

type Block struct {
	Start int
	Size  int
	PID   int
	Label string
}

func (b Block) Free() bool { return b.PID < 0 }
func (b Block) End() int   { return b.Start + b.Size }

type Page struct {
	Index      int
	Proc       *Process
	UsedBytes  int
	Present    bool
	Frame      int
	Referenced bool
	Dirty      bool
	LoadedAt   int
	LastUsed   int
	SwapSlot   int
}

type Process struct {
	PID   int
	Name  string
	Type  ProcType
	Owner string
	Size  int
	Base  int
	Pages []*Page
}

type Stats struct {
	Refs         int
	Hits         int
	Faults       int
	Replacements int
	SwapIns      int
	SwapOuts     int
	CleanDrops   int
	DemandLoads  int
	Compactions  int
}

type Manager struct {
	ReservedSize int
	Blocks       []Block
	Fit          FitPolicy

	NumFrames int
	Frames    []*Page
	SwapSlots int
	swapUsed  []bool
	Algo      Algorithm
	hand      int

	Procs   map[int]*Process
	nextPID int
	Clock   int
	Stats   Stats

	ReportEvery int
	ops         int

	Quiet bool

	SwapIO func(write bool, slot int) int
}

func NewManager() *Manager {
	m := &Manager{
		ReservedSize: DefaultReservedSize,
		Fit:          FirstFit,
		NumFrames:    DefaultFrames,
		SwapSlots:    DefaultSwapSlots,
		Algo:         LRU,
		Procs:        map[int]*Process{},
		nextPID:      FirstUserPID,
	}
	m.Frames = make([]*Page, m.NumFrames)
	m.swapUsed = make([]bool, m.SwapSlots)
	m.Blocks = []Block{
		{Start: 0, Size: KernelBaseSize, PID: 0, Label: "Núcleo Base do SO"},
		{Start: KernelBaseSize, Size: m.ReservedSize - KernelBaseSize, PID: -1, Label: "livre"},
	}
	return m
}

func (m *Manager) DynamicBase() int { return m.ReservedSize }

// ---------- Saída ----------

func (m *Manager) pf(format string, a ...interface{}) {
	if !m.Quiet {
		fmt.Printf(format, a...)
	}
}

// ---------- Utilidades de formatação ----------

func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func fmtMB(b int) string {
	s := trimFloat(float64(b) / MB)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s + " MB"
}

func fmtKB(b int) string {
	if b%KB == 0 {
		return fmt.Sprintf("%d KB", b/KB)
	}
	return trimFloat(float64(b)/KB) + " KB"
}

func ParseSize(s string) (int, error) {
	t := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	mult := 1
	switch {
	case strings.HasSuffix(t, "MB"):
		mult = MB
		t = strings.TrimSuffix(t, "MB")
	case strings.HasSuffix(t, "KB"):
		mult = KB
		t = strings.TrimSuffix(t, "KB")
	case strings.HasSuffix(t, "B"):
		t = strings.TrimSuffix(t, "B")
	}
	v, err := strconv.ParseFloat(t, 64)
	n := int(v * float64(mult))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("tamanho inválido: '%s' (use ex.: 512KB, 2MB, 10240B)", s)
	}
	return n, nil
}

func pagesText(n int) string {
	switch n {
	case 1:
		return "Página 0"
	case 2:
		return "Páginas 0 e 1"
	}
	parts := make([]string, 0, n-1)
	for i := 0; i < n-1; i++ {
		parts = append(parts, strconv.Itoa(i))
	}
	return "Páginas " + strings.Join(parts, ", ") + " e " + strconv.Itoa(n-1)
}

func rangesText(idx []int) string {
	if len(idx) == 0 {
		return "-"
	}
	var parts []string
	start, prev := idx[0], idx[0]
	flush := func() {
		if start == prev {
			parts = append(parts, strconv.Itoa(start))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", start, prev))
		}
	}
	for _, v := range idx[1:] {
		if v == prev+1 {
			prev = v
			continue
		}
		flush()
		start, prev = v, v
	}
	flush()
	return strings.Join(parts, ", ")
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---------- Processos ----------

func (m *Manager) sortedProcs() []*Process {
	list := make([]*Process, 0, len(m.Procs))
	for _, p := range m.Procs {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].PID < list[j].PID })
	return list
}

func (m *Manager) normalProcs() []*Process {
	var list []*Process
	for _, p := range m.sortedProcs() {
		if p.Type == Normal {
			list = append(list, p)
		}
	}
	return list
}

func (m *Manager) hasNormal() bool { return len(m.normalProcs()) > 0 }

func (m *Manager) nameTaken(name string) bool {
	for _, p := range m.Procs {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

func (m *Manager) Lookup(ref string) *Process {
	ref = strings.TrimSpace(ref)
	if pid, err := strconv.Atoi(ref); err == nil {
		if p, ok := m.Procs[pid]; ok {
			return p
		}
	}
	for _, p := range m.sortedProcs() {
		if strings.EqualFold(p.Name, ref) {
			return p
		}
	}
	return nil
}

func (m *Manager) totalPages() int {
	n := 0
	for _, p := range m.normalProcs() {
		n += len(p.Pages)
	}
	return n
}

// ---------- Configuração ----------

func (m *Manager) ResizeFrames(n int) error {
	if n < 1 || n > 4096 {
		return fmt.Errorf("número de molduras inválido (use 1..4096)")
	}
	if m.hasNormal() {
		return fmt.Errorf("há processos normais ativos; encerre-os antes de redimensionar a RAM dinâmica")
	}
	m.NumFrames = n
	m.Frames = make([]*Page, n)
	m.hand = 0
	return nil
}

func (m *Manager) ResizeSwap(n int) error {
	if n < 1 || n > 65536 {
		return fmt.Errorf("número de slots de swap inválido (use 1..65536)")
	}
	if m.hasNormal() {
		return fmt.Errorf("há processos normais ativos; encerre-os antes de redimensionar o swap")
	}
	m.SwapSlots = n
	m.swapUsed = make([]bool, n)
	return nil
}

func (m *Manager) afterOp() {
	m.ops++
	if m.ReportEvery > 0 && m.ops%m.ReportEvery == 0 && !m.Quiet {
		fmt.Printf("\n[RELATÓRIO PERIÓDICO DE FRAGMENTAÇÃO - após %d operações de memória]", m.ops)
		m.PrintFragReport()
	}
}
