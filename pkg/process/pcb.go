package process

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"

	"guga-fs/pkg/memory"
)

type State string

const (
	StateNew        State = "NOVO"
	StateReady      State = "PRONTO"
	StateRunning    State = "EXECUTANDO"
	StateBlocked    State = "BLOQUEADO"
	StateTerminated State = "CONCLUÍDO"
)

type SyscallProfile struct {
	Enabled   bool
	Random    bool
	Name      string
	TriggerAt int
	Duration  int
	Triggered bool
}

func (s SyscallProfile) Summary() string {
	if !s.Enabled {
		return "Nenhuma (CPU-bound)"
	}
	if s.Random {
		return fmt.Sprintf("Evento Aleatório Habilitado (disparo previsto em t=%dms, duração: %dms)", s.TriggerAt, s.Duration)
	}
	return fmt.Sprintf("Agendada para t=%dms (%s | Duração do Bloqueio: %dms)", s.TriggerAt, s.Name, s.Duration)
}

var syscallRegex = regexp.MustCompile(`(?i)^\s*([a-z0-9_-]+)\s+at\s+(\d+)\s*(?:ms)?\s*(?:\(\s*duration\s*:\s*(\d+)\s*(?:ms)?\s*\))?\s*$`)

func ParseSyscall(spec string, burst int, rng *rand.Rand) (SyscallProfile, error) {
	raw := strings.TrimSpace(spec)
	if raw == "" || strings.EqualFold(raw, "none") || strings.EqualFold(raw, "nenhuma") {
		return SyscallProfile{Enabled: false}, nil
	}
	if strings.EqualFold(raw, "random") || strings.EqualFold(raw, "aleatorio") || strings.EqualFold(raw, "aleatório") {
		trigger := 1
		if burst > 1 {
			trigger = 1 + rng.Intn(burst-1)
		}
		dur := 2 + rng.Intn(4) // 2ms a 5ms
		return SyscallProfile{
			Enabled:   true,
			Random:    true,
			Name:      "random_io",
			TriggerAt: trigger,
			Duration:  dur,
		}, nil
	}

	m := syscallRegex.FindStringSubmatch(raw)
	if m == nil {
		return SyscallProfile{}, fmt.Errorf("formato de syscall inválido: '%s' (use ex.: \"read_disk at 3ms (duration: 4ms)\" ou \"random\")", spec)
	}
	name := m[1]
	at, _ := strconv.Atoi(m[2])
	dur := 3
	if m[3] != "" {
		dur, _ = strconv.Atoi(m[3])
	}
	if at < 0 || dur <= 0 {
		return SyscallProfile{}, fmt.Errorf("tempo ou duração de syscall inválidos em '%s'", spec)
	}
	return SyscallProfile{
		Enabled:   true,
		Random:    false,
		Name:      name,
		TriggerAt: at,
		Duration:  dur,
	}, nil
}

type Thread struct {
	TID          int
	Name         string
	ParentPID    int
	State        State
	WaitingMutex string
}

type Mutex struct {
	Name      string
	Locked    bool
	Owner     *Thread
	WaitQueue []*Thread
}

type PCB struct {
	PID           int
	Name          string
	Owner         string
	Type          memory.ProcType
	MemProc       *memory.Process
	Priority      int
	State         State
	ArrivalTime   int
	BurstTime     int
	RemainingTime int
	ExecutedTime  int
	BlockedUntil  int
	BlockReason   string
	CompletionAt  int
	FirstRunAt    int
	Syscall       SyscallProfile
	Threads       []*Thread
	nextTID       int
	pageCursor    int
}

func (p *PCB) PriorityLabel() string {
	switch {
	case p.Priority <= 1:
		return fmt.Sprintf("%d (Alta)", p.Priority)
	case p.Priority == 2:
		return fmt.Sprintf("%d (Baixa)", p.Priority)
	default:
		return fmt.Sprintf("%d (Muito Baixa)", p.Priority)
	}
}

func (p *PCB) Transition(next State, reason string) {
	if p.State == next {
		return
	}
	p.State = next
	p.BlockReason = reason
}

func (p *PCB) NextLogicalPage() int {
	if p.MemProc == nil || len(p.MemProc.Pages) == 0 {
		return 0
	}
	n := len(p.MemProc.Pages)
	pattern := []int{0, 1, 2, 1, 3, 0, 4, 2}
	idx := pattern[p.pageCursor%len(pattern)] % n
	p.pageCursor++
	return idx
}

func (p *PCB) AddThread(name string) *Thread {
	if p.nextTID == 0 {
		p.nextTID = 1
	}
	if name == "" {
		name = fmt.Sprintf("T%d", p.nextTID)
	}
	t := &Thread{
		TID:       p.nextTID,
		Name:      name,
		ParentPID: p.PID,
		State:     StateReady,
	}
	p.nextTID++
	p.Threads = append(p.Threads, t)
	return t
}

func (p *PCB) FindThread(ref string) *Thread {
	for _, t := range p.Threads {
		if strings.EqualFold(t.Name, ref) || strconv.Itoa(t.TID) == ref {
			return t
		}
	}
	return nil
}
