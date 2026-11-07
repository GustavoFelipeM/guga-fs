package storage

import (
	"fmt"
	"sort"
	"strings"
)

type Algorithm string

const (
	FCFS  Algorithm = "FCFS"
	SSTF  Algorithm = "SSTF"
	SCAN  Algorithm = "SCAN"
	CSCAN Algorithm = "C-SCAN"
)

const CountCSCANReturn = true

var algoNames = map[Algorithm]string{
	FCFS:  "FCFS (First Come, First Served)",
	SSTF:  "SSTF (Shortest Seek Time First)",
	SCAN:  "SCAN (Elevador)",
	CSCAN: "C-SCAN (Elevador Circular)",
}

func ParseAlgorithm(s string) (Algorithm, bool) {
	switch strings.ToLower(strings.ReplaceAll(s, "-", "")) {
	case "fcfs":
		return FCFS, true
	case "sstf":
		return SSTF, true
	case "scan":
		return SCAN, true
	case "cscan":
		return CSCAN, true
	}
	return "", false
}

func AllAlgorithms() []Algorithm {
	return []Algorithm{FCFS, SSTF, SCAN, CSCAN}
}

type Visit struct {
	Track     int
	IsRequest bool
	IsReturn  bool
}

type Result struct {
	Algo   Algorithm
	Visits []Visit
	Moves  []int
	Total  int
}

func (r Result) FinalHead() int {
	return r.Visits[len(r.Visits)-1].Track
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func Schedule(algo Algorithm, head int, requests []int, maxTrack int, up bool) Result {
	visits := []Visit{{Track: head, IsRequest: false}}
	add := func(t int, isReq, isReturn bool) {
		if t == visits[len(visits)-1].Track && !isReturn {

			if isReq {
				visits[len(visits)-1].IsRequest = true
			}
			return
		}
		visits = append(visits, Visit{Track: t, IsRequest: isReq, IsReturn: isReturn})
	}
	last := func() int { return visits[len(visits)-1].Track }

	reqs := append([]int(nil), requests...)

	switch algo {
	case FCFS:
		for _, r := range reqs {
			add(r, true, false)
		}

	case SSTF:
		remaining := reqs
		for len(remaining) > 0 {
			best := 0
			for i, r := range remaining {
				if abs(r-last()) < abs(remaining[best]-last()) {
					best = i
				}
			}
			add(remaining[best], true, false)
			remaining = append(remaining[:best], remaining[best+1:]...)
		}

	case SCAN, CSCAN:
		asc := append([]int(nil), reqs...)
		sort.Ints(asc)
		desc := append([]int(nil), asc...)
		sort.Sort(sort.Reverse(sort.IntSlice(desc)))

		pick := func(src []int, keep func(int) bool) []int {
			out := []int{}
			for _, t := range src {
				if keep(t) {
					out = append(out, t)
				}
			}
			return out
		}

		if algo == SCAN {
			var first, second []int
			edge := maxTrack
			if up {
				first = pick(asc, func(t int) bool { return t >= head })
				second = pick(desc, func(t int) bool { return t < head })
			} else {
				first = pick(desc, func(t int) bool { return t <= head })
				second = pick(asc, func(t int) bool { return t > head })
				edge = 0
			}
			for _, t := range first {
				add(t, true, false)
			}
			if len(second) > 0 {
				add(edge, false, false)
				for _, t := range second {
					add(t, true, false)
				}
			}
		} else {
			var first, second []int
			edge, other := maxTrack, 0
			if up {
				first = pick(asc, func(t int) bool { return t >= head })
				second = pick(asc, func(t int) bool { return t < head })
			} else {
				first = pick(desc, func(t int) bool { return t <= head })
				second = pick(desc, func(t int) bool { return t > head })
				edge, other = 0, maxTrack
			}
			for _, t := range first {
				add(t, true, false)
			}
			if len(second) > 0 {
				add(edge, false, false)
				add(other, false, true)
				for _, t := range second {
					add(t, true, false)
				}
			}
		}
	}

	res := Result{Algo: algo, Visits: visits}
	for i := 1; i < len(visits); i++ {
		cost := abs(visits[i].Track - visits[i-1].Track)
		if visits[i].IsReturn && !CountCSCANReturn {
			cost = 0
		}
		res.Moves = append(res.Moves, cost)
		res.Total += cost
	}
	return res
}

func (r Result) Print() {
	fmt.Printf("\n--- ESCALONAMENTO DE DISCO: %s ---\n", algoNames[r.Algo])

	order := make([]string, len(r.Visits))
	hasEdge := false
	for i, v := range r.Visits {
		order[i] = fmt.Sprintf("%d", v.Track)
		if !v.IsRequest && i > 0 {
			order[i] += "*"
			hasEdge = true
		}
	}
	fmt.Printf("Ordem de Atendimento : %s\n", strings.Join(order, " -> "))

	if len(r.Moves) == 0 {
		fmt.Println("Deslocamento da Cabeça: (nenhuma requisição)")
		fmt.Println("Total de Movimento    : 0 trilhas")
		return
	}

	terms := make([]string, len(r.Moves))
	costs := make([]string, len(r.Moves))
	for i := range r.Moves {
		a, b := r.Visits[i].Track, r.Visits[i+1].Track
		terms[i] = fmt.Sprintf("|%d-%d|", a, b)
		if r.Visits[i+1].IsReturn {
			terms[i] += " (retorno)"
		}
		costs[i] = fmt.Sprintf("%d", r.Moves[i])
	}
	fmt.Printf("Deslocamento da Cabeça: %s\n", strings.Join(terms, " + "))
	fmt.Printf("Total de Movimento    : %s = %d trilhas\n", strings.Join(costs, " + "), r.Total)
	if hasEdge {
		fmt.Println("(* = extremidade do disco, visitada apenas para inverter/retornar)")
	}
}

func Compare(head int, requests []int, maxTrack int, up bool) {
	fmt.Printf("\n=== COMPARATIVO (cabeça na trilha %d, disco 0..%d) ===\n", head, maxTrack)
	best := -1
	results := []Result{}
	for _, a := range AllAlgorithms() {
		r := Schedule(a, head, requests, maxTrack, up)
		results = append(results, r)
		if best == -1 || r.Total < best {
			best = r.Total
		}
	}
	for _, r := range results {
		mark := ""
		if r.Total == best {
			mark = "  <- menor deslocamento"
		}
		fmt.Printf("%-8s: %4d trilhas%s\n", r.Algo, r.Total, mark)
	}
}
