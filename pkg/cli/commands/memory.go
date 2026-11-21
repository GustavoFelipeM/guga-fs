package commands

import (
	"fmt"
	"strconv"
	"strings"

	"guga-fs/pkg/fs"
	"guga-fs/pkg/memory"
	"guga-fs/pkg/storage"
)

const createProcessUsage = `Uso: create_process --name <nome> --size <tamanho> [--type critical|normal]
  --type critical   Área Reservada (alocação contígua, sem paginação) - exige Modo Kernel
  --type normal     Memória Virtual (paginada, por demanda) - padrão
  Exemplos: create_process --name "Kernel_DiskDriver" --size 512KB --type critical
            create_process --name "App_Navegador" --size 10KB --type normal`

const memoryConfigUsage = `Uso: memory_config [opções]   (sem opções mostra a configuração; alterar exige Modo Kernel)
  --algo <lru|second_chance|fifo>   algoritmo de substituição de páginas
  --frames <N>                      nº de molduras da RAM dinâmica (sem processos normais ativos)
  --swap <N>                        nº de slots de swap = páginas de memória virtual (idem)
  --fit <first|best|worst>          política de alocação contígua na Área Reservada
  --report-every <N>                relatório de fragmentação a cada N operações (0 = desliga)`

// ---------- utilidades ----------

type flagSet struct {
	vals map[string]string
	seen map[string]bool
}

func parseFlags(args []string) flagSet {
	f := flagSet{vals: map[string]string{}, seen: map[string]bool{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") || len(a) == 2 {
			continue
		}
		name, val, hasEq := strings.Cut(a[2:], "=")
		name = strings.ToLower(name)
		f.seen[name] = true
		if hasEq {
			f.vals[name] = strings.Trim(val, "\"'")
			continue
		}
		var parts []string
		for i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			parts = append(parts, args[i+1])
			i++
		}
		f.vals[name] = strings.Trim(strings.Join(parts, " "), "\"'")
	}
	return f
}

func (f flagSet) get(name string) string { return f.vals[name] }
func (f flagSet) isSet(name string) bool { return f.seen[name] }

func parseInt(s string) (int, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 0, 64)
	if err != nil {
		return 0, false
	}
	return int(n), true
}

func getMem(vfs *fs.VirtualFileSystem) *memory.Manager {
	if vfs.Mem == nil {
		fmt.Println("Memória: gerenciador não inicializado.")
		return nil
	}
	return vfs.Mem
}

func canTouch(vfs *fs.VirtualFileSystem, p *memory.Process, op string) bool {
	if vfs.Sec.IsKernel() || p.Owner == vfs.Sec.CurrentName() {
		return true
	}
	fmt.Printf("\n[ACESSO NEGADO] O processo '%s' (PID %d) pertence a '%s'; '%s' não pode executar '%s' sobre ele.\n",
		p.Name, p.PID, p.Owner, vfs.Sec.CurrentName(), op)
	fmt.Println("Tentativa não autorizada registrada nos logs do sistema.")
	vfs.Sec.Record("MEMORIA_ACESSO_NEGADO", false, fmt.Sprintf("%s pid=%d", op, p.PID))
	return false
}

// ---------- create_process / terminate_process ----------

func HandleCreateProcess(vfs *fs.VirtualFileSystem, args []string) {
	mem := getMem(vfs)
	if mem == nil || !requireLogin(vfs) {
		return
	}
	f := parseFlags(args)
	name, sizeStr := f.get("name"), f.get("size")
	if name == "" || sizeStr == "" {
		fmt.Println(createProcessUsage)
		return
	}
	size, err := memory.ParseSize(sizeStr)
	if err != nil {
		fmt.Println("create_process:", err)
		return
	}
	typ := strings.ToLower(f.get("type"))
	if typ == "" {
		typ = "normal"
	}
	owner := vfs.Sec.CurrentName()

	var p *memory.Process
	switch typ {
	case "critical", "critico", "crítico":
		if !vfs.Sec.RequireKernel("create_process --type critical") {
			return
		}
		p = mem.CreateCritical(name, owner, size)
	case "normal", "user", "usuario", "usuário":
		p = mem.CreateNormal(name, owner, size)
	default:
		fmt.Printf("create_process: tipo '%s' inválido (use critical ou normal).\n", typ)
		return
	}
	if p != nil {
		vfs.Sec.Record("CREATE_PROCESS", true, fmt.Sprintf("%s pid=%d tipo=%s tamanho=%d", p.Name, p.PID, p.Type, p.Size))
	}
}

func HandleTerminateProcess(vfs *fs.VirtualFileSystem, args []string) {
	mem := getMem(vfs)
	if mem == nil || !requireLogin(vfs) {
		return
	}
	f := parseFlags(args)
	ref := f.get("name")
	if ref == "" {
		ref = f.get("pid")
	}
	if ref == "" {
		fmt.Println("Uso: terminate_process --name <nome> | --pid <PID>")
		return
	}
	p := mem.Lookup(ref)
	if p == nil {
		fmt.Printf("terminate_process: processo '%s' não encontrado.\n", ref)
		return
	}
	if p.Type == memory.Critical {
		if !vfs.Sec.RequireKernel("terminate_process (processo crítico)") {
			return
		}
	} else if !canTouch(vfs, p, "terminate_process") {
		return
	}
	vfs.Sec.Record("TERMINATE_PROCESS", true, fmt.Sprintf("%s pid=%d tipo=%s", p.Name, p.PID, p.Type))
	mem.Terminate(p)
}

// ---------- memory_status / memory_compact ----------

func HandleMemoryStatus(vfs *fs.VirtualFileSystem, args []string) {
	mem := getMem(vfs)
	if mem == nil {
		return
	}
	f := parseFlags(args)
	reserved, virtual := f.isSet("reserved"), f.isSet("virtual")
	switch {
	case f.isSet("all") || (reserved && virtual):
		mem.PrintReserved()
		mem.PrintVirtual()
		mem.PrintUsage()
	case reserved:
		mem.PrintReserved()
	case virtual:
		mem.PrintVirtual()
	default:
		mem.PrintUsage()
		mem.PrintFragReport()
		if c, ok := vfs.IO.(*storage.Controller); ok && c != nil {
			fmt.Printf("Fila do disco (Etapa 2)   : %d requisição(ões) de fundo/swap aguardando o escalonador (próxima E/S do FS as processa)\n", len(c.Background))
		}
	}
}

func HandleMemoryCompact(vfs *fs.VirtualFileSystem, args []string) {
	mem := getMem(vfs)
	if mem == nil {
		return
	}
	f := parseFlags(args)
	if f.isSet("virtual") && !f.isSet("reserved") {
		fmt.Println("memory_compact: a região paginada não sofre fragmentação externa (qualquer moldura livre atende qualquer página); só a Área Reservada é compactada.")
		return
	}
	if !vfs.Sec.RequireKernel("memory_compact --reserved") {
		return
	}
	if mem.Compact() {
		vfs.Sec.Record("MEMORY_COMPACT", true, "área reservada")
	}
}

// ---------- translate_address / execute_step ----------

func HandleTranslateAddress(vfs *fs.VirtualFileSystem, args []string) {
	mem := getMem(vfs)
	if mem == nil || !requireLogin(vfs) {
		return
	}
	const usage = "Uso: translate_address --process <nome|PID> (--page <N> [--offset <bytes>] | --addr <endereço lógico>)"
	f := parseFlags(args)
	ref := f.get("process")
	if ref == "" {
		fmt.Println(usage)
		return
	}
	p := mem.Lookup(ref)
	if p == nil {
		fmt.Printf("translate_address: processo '%s' não encontrado.\n", ref)
		return
	}
	if !canTouch(vfs, p, "translate_address") {
		return
	}

	page, offset := 0, 0
	switch {
	case f.isSet("addr"):
		addr, ok := parseInt(f.get("addr"))
		if !ok || addr < 0 {
			fmt.Println("translate_address: endereço lógico inválido.")
			return
		}
		page, offset = addr/memory.PageSize, addr%memory.PageSize
	case f.isSet("page"):
		n, ok := parseInt(f.get("page"))
		if !ok {
			fmt.Println("translate_address: número de página inválido.")
			return
		}
		page = n
		if f.isSet("offset") {
			o, ok := parseInt(f.get("offset"))
			if !ok || o < 0 {
				fmt.Println("translate_address: deslocamento inválido.")
				return
			}
			offset = o
		}
	default:
		fmt.Println(usage)
		return
	}
	mem.Translate(p, page, offset)
}

func HandleExecuteStep(vfs *fs.VirtualFileSystem, args []string) {
	mem := getMem(vfs)
	if mem == nil || !requireLogin(vfs) {
		return
	}
	const usage = `Uso: execute_step --process <nome|PID> --instruction "LOAD_DATA Página 2"
     (instruções STORE/WRITE/SAVE/SET marcam a página como modificada; alternativa: --page <N> [--write])`
	f := parseFlags(args)
	ref := f.get("process")
	instr := f.get("instruction")
	if ref == "" || (instr == "" && !f.isSet("page")) {
		fmt.Println(usage)
		return
	}
	p := mem.Lookup(ref)
	if p == nil {
		fmt.Printf("execute_step: processo '%s' não encontrado.\n", ref)
		return
	}
	if !canTouch(vfs, p, "execute_step") {
		return
	}

	opcode, page, found := "LOAD", 0, false
	if instr != "" {
		toks := strings.Fields(instr)
		opcode = strings.ToUpper(toks[0])
		for i := len(toks) - 1; i >= 1; i-- {
			if n, ok := parseInt(toks[i]); ok {
				page, found = n, true
				break
			}
		}
	}
	if f.isSet("page") {
		n, ok := parseInt(f.get("page"))
		if !ok {
			fmt.Println("execute_step: número de página inválido.")
			return
		}
		page, found = n, true
	}
	if !found && p.Type == memory.Normal {
		fmt.Println("execute_step: a instrução precisa citar o número da página (ex.: \"LOAD_DATA Página 2\").")
		return
	}
	write := f.isSet("write")
	for _, pre := range []string{"STORE", "WRITE", "SAVE", "SET"} {
		if strings.HasPrefix(opcode, pre) {
			write = true
		}
	}
	mem.Access(p, page, write)
}

// ---------- memory_config / memory_demo ----------

func HandleMemoryConfig(vfs *fs.VirtualFileSystem, args []string) {
	mem := getMem(vfs)
	if mem == nil {
		return
	}
	f := parseFlags(args)
	if len(f.seen) == 0 {
		mem.PrintConfig()
		return
	}
	if !f.isSet("algo") && !f.isSet("fit") && !f.isSet("frames") && !f.isSet("swap") && !f.isSet("report-every") {
		fmt.Println(memoryConfigUsage)
		return
	}
	if !vfs.Sec.RequireKernel("memory_config") {
		return
	}

	if f.isSet("algo") {
		a, ok := memory.ParseAlgorithm(f.get("algo"))
		if !ok {
			fmt.Println("memory_config: algoritmo inválido (use lru, second_chance ou fifo).")
			return
		}
		mem.Algo = a
	}
	if f.isSet("fit") {
		p, ok := memory.ParseFit(f.get("fit"))
		if !ok {
			fmt.Println("memory_config: política inválida (use first, best ou worst).")
			return
		}
		mem.Fit = p
	}
	if f.isSet("frames") {
		n, ok := parseInt(f.get("frames"))
		if !ok {
			fmt.Println("memory_config: --frames precisa de um número.")
			return
		}
		if err := mem.ResizeFrames(n); err != nil {
			fmt.Println("memory_config:", err)
			return
		}
	}
	if f.isSet("swap") {
		n, ok := parseInt(f.get("swap"))
		if !ok {
			fmt.Println("memory_config: --swap precisa de um número.")
			return
		}
		if err := mem.ResizeSwap(n); err != nil {
			fmt.Println("memory_config:", err)
			return
		}
	}
	if f.isSet("report-every") {
		n, ok := parseInt(f.get("report-every"))
		if !ok || n < 0 {
			fmt.Println("memory_config: --report-every precisa de um número >= 0.")
			return
		}
		mem.ReportEvery = n
	}
	vfs.Sec.Record("MEMORY_CONFIG", true, strings.Join(args, " "))
	mem.PrintConfig()
}

func HandleMemoryDemo(vfs *fs.VirtualFileSystem, args []string) {
	memory.Demo()
}
