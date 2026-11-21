package process

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"guga-fs/pkg/memory"
	"guga-fs/pkg/storage"
)

const (
	DefaultBurst    = 6
	DefaultPriority = 2
	DefaultProcSize = 32 * memory.KB
)

type CreateOptions struct {
	Name        string
	Owner       string
	Size        int
	Type        memory.ProcType
	Burst       int
	Priority    int
	SyscallSpec string
	ArrivalTime int
}

type Manager struct {
	Mem     *memory.Manager
	IO      *storage.Controller
	Procs   map[int]*PCB
	Mutexes map[string]*Mutex
	Clock   int
	rng     *rand.Rand
}

func NewManager(mem *memory.Manager, io *storage.Controller) *Manager {
	return &Manager{
		Mem:     mem,
		IO:      io,
		Procs:   make(map[int]*PCB),
		Mutexes: make(map[string]*Mutex),
		rng:     rand.New(rand.NewSource(42)),
	}
}

func (m *Manager) Create(opts CreateOptions) (*PCB, error) {
	if opts.Size <= 0 {
		opts.Size = DefaultProcSize
	}
	if opts.Burst <= 0 {
		opts.Burst = DefaultBurst
	}
	if opts.Priority <= 0 {
		if opts.Type == memory.Critical {
			opts.Priority = 1
		} else {
			opts.Priority = DefaultPriority
		}
	}

	sc, err := ParseSyscall(opts.SyscallSpec, opts.Burst, m.rng)
	if err != nil {
		return nil, err
	}

	prevQuiet := m.Mem.Quiet
	m.Mem.Quiet = true
	var mp *memory.Process
	if opts.Type == memory.Critical {
		mp = m.Mem.CreateCritical(opts.Name, opts.Owner, opts.Size)
	} else {
		mp = m.Mem.CreateNormal(opts.Name, opts.Owner, opts.Size)
	}
	m.Mem.Quiet = prevQuiet

	if mp == nil {
		return nil, fmt.Errorf("falha ao alocar memória para o processo '%s'", opts.Name)
	}

	pcb := &PCB{
		PID:           mp.PID,
		Name:          mp.Name,
		Owner:         mp.Owner,
		Type:          mp.Type,
		MemProc:       mp,
		Priority:      opts.Priority,
		State:         StateNew,
		ArrivalTime:   opts.ArrivalTime,
		BurstTime:     opts.Burst,
		RemainingTime: opts.Burst,
		FirstRunAt:    -1,
		CompletionAt:  -1,
		Syscall:       sc,
	}
	pcb.AddThread("T1")

	pcb.Transition(StateReady, "")
	m.Procs[pcb.PID] = pcb

	allocDesc := "Memória Virtual Paginada (Tabela de Páginas criada)"
	if pcb.Type == memory.Critical {
		allocDesc = fmt.Sprintf("Área Reservada Contígua (Base 0x%X, %d KB)", mp.Base, mp.Size/memory.KB)
	}

	fmt.Printf("\n[CRIAÇÃO DE PROCESSO] PID: %d | Nome: %s | Prioridade: %s\n", pcb.PID, pcb.Name, pcb.PriorityLabel())
	fmt.Printf("Ciclo de Vida : %s -> %s\n", StateNew, pcb.State)
	fmt.Printf("Alocação      : %s\n", allocDesc)
	fmt.Printf("Burst CPU     : %dms | Dono: %s\n", pcb.BurstTime, pcb.Owner)
	if pcb.Syscall.Enabled {
		if pcb.Syscall.Random {
			fmt.Printf("Syscall/I/O   : Evento Aleatório Habilitado (gatilho em t=%dms, duração: %dms)\n",
				pcb.Syscall.TriggerAt, pcb.Syscall.Duration)
		} else {
			fmt.Printf("Syscall/I/O   : Agendada para t=%dms (Duração do Bloqueio: %dms)\n",
				pcb.Syscall.TriggerAt, pcb.Syscall.Duration)
		}
	} else {
		fmt.Printf("Syscall/I/O   : Nenhuma\n")
	}
	fmt.Printf("Estado Inicial: %s\n", pcb.State)

	return pcb, nil
}

func (m *Manager) RegisterExisting(mp *memory.Process) *PCB {
	if mp == nil {
		return nil
	}
	if existing, ok := m.Procs[mp.PID]; ok {
		return existing
	}
	prio := DefaultPriority
	if mp.Type == memory.Critical {
		prio = 1
	}
	pcb := &PCB{
		PID:           mp.PID,
		Name:          mp.Name,
		Owner:         mp.Owner,
		Type:          mp.Type,
		MemProc:       mp,
		Priority:      prio,
		State:         StateReady,
		BurstTime:     DefaultBurst,
		RemainingTime: DefaultBurst,
		FirstRunAt:    -1,
		CompletionAt:  -1,
	}
	pcb.AddThread("T1")
	m.Procs[pcb.PID] = pcb
	return pcb
}

func (m *Manager) Remove(pid int) {
	delete(m.Procs, pid)
}

func (m *Manager) Lookup(ref string) *PCB {
	ref = strings.TrimSpace(ref)
	if pid, err := strconv.Atoi(ref); err == nil {
		if p, ok := m.Procs[pid]; ok {
			return p
		}
	}
	for _, p := range m.SortedProcs() {
		if strings.EqualFold(p.Name, ref) {
			return p
		}
	}
	if m.Mem != nil {
		if mp := m.Mem.Lookup(ref); mp != nil {
			return m.RegisterExisting(mp)
		}
	}
	return nil
}

func (m *Manager) SortedProcs() []*PCB {
	if m.Mem != nil {
		for _, mp := range m.Mem.Procs {
			if _, ok := m.Procs[mp.PID]; !ok {
				m.RegisterExisting(mp)
			}
		}
	}
	list := make([]*PCB, 0, len(m.Procs))
	for _, p := range m.Procs {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].PID < list[j].PID })
	return list
}

func (m *Manager) enqueueDiskSyscall(p *PCB) int {
	if m.IO == nil {
		return -1
	}
	track := (p.PID*17 + p.ExecutedTime*7) % 150 // trilhas de dados (0..149)
	m.IO.Background = append(m.IO.Background, track)
	return track
}

// ---------- Concorrência: Threads e Mutex ----------

func (m *Manager) getOrCreateMutex(name string) *Mutex {
	mx, ok := m.Mutexes[name]
	if !ok {
		mx = &Mutex{Name: name}
		m.Mutexes[name] = mx
	}
	return mx
}

func (m *Manager) LockMutex(p *PCB, threadRef, mutexName string, atTime int) bool {
	t := p.FindThread(threadRef)
	if t == nil {
		t = p.AddThread(threadRef)
	}
	mx := m.getOrCreateMutex(mutexName)

	fmt.Printf("[t=%dms] Thread %s (PID %d) tenta acessar Região Crítica ('%s').\n",
		atTime, t.Name, p.PID, mutexName)

	if !mx.Locked {
		mx.Locked = true
		mx.Owner = t
		t.State = StateRunning
		t.WaitingMutex = ""
		fmt.Printf("        Mutex [LIVRE] -> %s adquire o Mutex [BLOQUEADO].\n", t.Name)
		return true
	}

	if mx.Owner == t {
		fmt.Printf("        Mutex já pertence a %s.\n", t.Name)
		return true
	}

	prev := t.State
	t.State = StateBlocked
	t.WaitingMutex = mutexName
	mx.WaitQueue = append(mx.WaitQueue, t)
	p.Transition(StateBlocked, fmt.Sprintf("Aguardando Mutex '%s'", mutexName))

	fmt.Printf("        Mutex [BLOQUEADO por %s]!\n", mx.Owner.Name)
	fmt.Printf("        - Mudança de Estado de %s: %s -> %s (Aguardando Mutex).\n", t.Name, prev, t.State)
	return false
}

func (m *Manager) UnlockMutex(p *PCB, threadRef, mutexName string, atTime int) {
	mx, ok := m.Mutexes[mutexName]
	if !ok || !mx.Locked {
		fmt.Printf("[t=%dms] Mutex '%s' já está livre.\n", atTime, mutexName)
		return
	}
	t := p.FindThread(threadRef)
	if t == nil || mx.Owner != t {
		fmt.Printf("[t=%dms] Erro: Thread '%s' não detém a posse do Mutex '%s' (dono atual: %s).\n",
			atTime, threadRef, mutexName, mx.Owner.Name)
		return
	}

	fmt.Printf("[t=%dms] Thread %s (PID %d) libera Região Crítica ('%s').\n",
		atTime, t.Name, p.PID, mutexName)

	if len(mx.WaitQueue) > 0 {
		next := mx.WaitQueue[0]
		mx.WaitQueue = mx.WaitQueue[1:]
		mx.Owner = next
		next.WaitingMutex = ""
		next.State = StateReady
		if ownerPCB, exists := m.Procs[next.ParentPID]; exists {
			ownerPCB.Transition(StateReady, "")
		}
		fmt.Printf("        - Mutex transferido para Thread %s (PID %d): BLOQUEADO -> PRONTO.\n",
			next.Name, next.ParentPID)
		return
	}

	mx.Locked = false
	mx.Owner = nil
	fmt.Printf("        - Mutex '%s' agora está [LIVRE].\n", mutexName)
}
