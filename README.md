# Simulador de Sistema de Arquivos (Virtual File System - VFS)

- **Disciplina:** Sistemas Operacionais (BCC)
- **Professor:** Douglas Veras e Silva
- **Linguagem:** Go (Golang)


## Sobre o Projeto

Este projeto é um **Simulador de Sistema de Arquivos Virtual (VFS)** completo desenvolvido em **Go**, integrando desde a gestão de diretórios lógicos em árvore e diagnósticos de disco até a simulação física de **Escalonamento de E/S no Braço do Disco** e arranjos de **Redundância e Tolerância a Falhas via RAID**.

O simulador conta com um terminal interativo (REPL) com suporte a cores ANSI, comandos no estilo Unix/Linux (`cd`, `ls`, `mkdir`, `create`, `write_file`, `cat`, `rm`, `tree`, `chmod`, `fs_status`, `iosched`, `raid`), suporte a caminhos absolutos/relativos e tradução física de operações de leitura/escrita em requisições de disco.



## Arquitetura do Código (Modular)

O projeto segue uma arquitetura separada em camadas: o motor do sistema de arquivos (`fs`), a camada física de armazenamento e controle de E/S (`storage`), e a interface CLI interativa (`cli`):

```text
guga-fs/
├── main.go                     # Ponto de entrada da aplicação
├── go.mod                      # Módulo Go
├── demo_etapa2.txt             # Script de demonstração da Etapa 2
├── demo_etapa3.txt             # Script de demonstração da Etapa 3
└── pkg/
    ├── fs/                     # Núcleo do Sistema de Arquivos (Engine)
    │   ├── inode.go            # Estruturas de Inode, Blocos e Metadados
    │   ├── disk.go             # Inicialização, Resolução de Caminhos e Parse de Tamanhos
    │   ├── allocation.go       # Operações CRUD, Árvore e Relatório de Fragmentação
    │   └── security_ops.go     # ACL, chmod/chown, criptografia em repouso e antivírus (Etapa 3)
    ├── security/               # Segurança e Proteção (Etapa 3)
    │   ├── auth.go             # Usuários, grupos, SHA-256+salt, sessão, Modo Usuário/Kernel, auditoria
    │   ├── acl.go              # Permissões rwx (dono/grupo/outros) + ACL estendida
    │   ├── crypto.go           # Derivação de chave (SHA-256+salt) e AES-256-CTR por bloco
    │   └── integrity.go        # Registro oficial de hashes por bloco e assinaturas maliciosas
    ├── storage/                # Camada de Armazenamento, E/S e RAID (Etapa 2)
    │   ├── scheduler.go        # FCFS, SSTF, SCAN e C-SCAN + cálculo do deslocamento
    │   ├── raid.go             # Arranjos RAID 0, 1, 4 e 5 (Paridade XOR e Reconstrução)
    │   └── controller.go       # Controlador de E/S (Fila, tradução bloco -> trilha)
    └── cli/                    # Interface do Terminal Interativo (REPL)
        ├── shell.go            # Loop principal do Terminal
        └── commands/           # Manipuladores modulares de comandos
            ├── registry.go     # Mapeamento e registro de comandos
            ├── mkdir.go        # Comando mkdir
            ├── create.go       # Comando create
            ├── write_file.go   # Comando write_file (E/S com RAID)
            ├── cd.go           # Comando cd
            ├── ls.go           # Comando ls
            ├── cat.go          # Comando cat
            ├── rm.go           # Comando rm
            ├── tree.go         # Comando tree
            ├── fs_status.go    # Comando fs_status
            ├── iosched.go      # Comando iosched (Escalonador de Disco)
            ├── raid.go         # Comando raid (Gerenciador de Arranjos RAID)
            ├── chmod.go        # Comando chmod
            └── clear.go        # Comando clear
```



## Funcionalidades Implementadas

### Etapa 1 — Sistema de Arquivos & Diagnóstico de Disco
1. **Hierarquia em Árvore:**
   - Navegação com suporte a caminhos absolutos (ex.: `/sistema/logs/app.log`) e relativos (`.`, `..`).
   - Visualização gráfica em árvore através do comando `tree`.

2. **Operações CRUD:**
   - **Create:** `mkdir` (diretórios) e `create` / `write_file` (arquivos com parâmetro `--size`).
   - **Read:** `cat` (exibe conteúdo) e `ls` (lista diretórios).
   - **Update:** Alteração de metadados/permissões (`chmod`) e sobrescrita de arquivos.
   - **Delete:** `rm` (remoção de arquivos e diretórios vazios com liberação automática de blocos).

3. **Mapeamento e Alocação Física (i-node):**
   - Disco virtual divido em blocos de tamanho fixo (padrão: 4 KB por bloco).
   - **Alocação Indexada**, onde cada arquivo possui um Inode alocado que aponta para seus blocos de dados.

4. **Diagnóstico e Relatório de Fragmentação (`fs_status`):**
   - **Mapa de Blocos:** Exibição da ocupação exata do disco (Inodes, Blocos de Dados e Blocos Livres).
   - **Fragmentação Interna:** Cálculo exato do desperdício no último bloco ocupado por cada arquivo.
   - **Fragmentação Externa & Compactação:** Avaliação de blocos dispersos e análise automatizada sobre necessidade de compactação.


### Etapa 2 — Escalonamento de E/S & Redundância RAID
1. **Integração FS → E/S:**
   - A camada `fs` interage com o disco através da interface `fs.IOBackend` (`WriteBlock`, `ReadBlock`, `FreeBlock`, `Flush`).
   - Cada leitura (`cat`) ou escrita (`write_file`) converte o bloco lógico em uma trilha física de disco através do cálculo:
  `trilha = (bloco * total_trilhas) / total_blocos` *(arredondado para baixo)*.

2. **Escalonamento do Braço de Disco (`iosched`):**
   - Algoritmos implementados: **FCFS**, **SSTF**, **SCAN** (Elevador) e **C-SCAN** (Elevador Circular).
   - Cálculo e exibição detalhada do deslocamento total da cabeça de leitura/escrita em trilhas.
   - Injeção de requisições externas na fila (`iosched queue add`) e simulação interativa com `iosched demo`.

3. **Redundância e Tolerância a Falhas via RAID (`raid`):**
   - Suporte aos arranjos **RAID 0** (Striping), **RAID 1** (Espelhamento), **RAID 4** (Paridade Dedicada) e **RAID 5** (Paridade Distribuída).
   - Simulação de falha física em discos (`raid fail N` / `--fail N`).
   - Reconstrução de dados em tempo real utilizando operações lógicas XOR para RAID 4/5 e cópia direta para RAID 1.
   - Migração dinâmica entre níveis de RAID sem perda de dados.

### Etapa 3 — Segurança e Proteção
1. **Modos de execução:** `MODO_USUARIO` e `MODO_KERNEL`. O login como `root` entra em Modo Kernel; os demais usuários em Modo Usuário. Operações críticas (`useradd`, `groupadd`, `usermod`, `chown`, `raid` (exceto `status`), `iosched` (exceto consultas/`demo`), `write_file --fail`, `disk_tamper`, `audit_log`, `antivirus_scan --repair`, `users --show-hash`) chamam `RequireKernel`; sem privilégio a operação é negada e registrada. Membros do grupo `admin` podem executar **um** comando em Modo Kernel com `sudo` (reautenticando a senha); `su` troca de usuário.
2. **Identidade e autenticação:** múltiplos usuários e grupos (primário + suplementares). Senhas guardadas como **SHA-256 iterado (10.000x) com salt aleatório de 16 bytes**, comparadas em tempo constante. Nenhuma senha em texto claro fica na memória do simulador. A senha é pedida sem eco quando o terminal permite.
3. **Controle de acesso:** cada i-node tem dono, grupo, modo `rwx` (dono/grupo/outros) e ACL estendida (`setfacl`/`getfacl`). Ordem de avaliação: dono → usuário nomeado na ACL → grupos → outros. Leitura/escrita/execução são verificadas em `cat`/`ls` (r), `write_file`/`create`/`mkdir`/`rm` (w no arquivo ou no diretório pai) e `cd` (x). Toda negação imprime o relatório de ACL e grava no log de auditoria. O Modo Kernel ignora a verificação (superusuário).
4. **Criptografia em repouso:** `encrypt_file` cifra com **AES-256-CTR** (chave derivada da senha com salt, IV distinto por bloco) **antes** de gravar nos blocos físicos RAID; o texto claro não fica no i-node. `cat` sem chave mostra só o ciphertext; com `--key` decifra apenas na exibição; chave errada é detectada por um verificador de chave.
5. **Integridade (antivírus simulado):** a cada gravação, o SHA-256 de cada bloco (i-node + dados) é guardado no registro oficial. `antivirus_scan` relê os blocos direto do arranjo, compara os hashes e procura assinaturas maliciosas. Com `--repair` (kernel) blocos adulterados são **restaurados pela redundância da Etapa 2** (XOR no RAID 4/5, espelho no RAID 1), validando o resultado contra o hash oficial, e arquivos infectados vão para quarentena (dono root, modo 000). `disk_tamper` simula a escrita direta no disco (ignora FS, ACL e paridade) para demonstrar a detecção.
6. **Integração:** a Etapa 3 envolve as Etapas 1 e 2 — todo comando do FS passa pela ACL, os blocos criptografados/íntegros trafegam pela camada de E/S (escalonador + RAID), e as operações de RAID/escalonador passam a exigir Modo Kernel.


## Decisões de Modelagem

- **Parâmetros do Disco:** O disco simulado possui **200 trilhas (0..199)**.
- **C-SCAN:** O salto de retorno da última à primeira trilha é computado no cálculo total de deslocamento (`CountCSCANReturn`).
- **SCAN:** A cabeça só se desloca até a borda física do disco (trilha 0 ou 199) quando necessita inverter o sentido de varredura.
- **Batch Processing:** As requisições geradas por um comando do sistema de arquivos são agrupadas e escalonadas conjuntamente durante o `Flush`.


- **Etapa 3:** `root` é criado no boot com senha inicial `root123` (apenas para a demonstração). Grupos iniciais: `admin`, `users`, `devs`. Arquivos novos nascem `644` e diretórios `755`, com o dono sendo o usuário da sessão. O `antivirus_scan` é um serviço do sistema (qualquer usuário autenticado pode pedir o relatório, que não exibe conteúdo); só o reparo exige Modo Kernel. O disco simulado continua com 16 blocos.

## Como Executar

### Pré-requisitos
- **Go** (versão 1.27 ou superior) instalado.

### Passos para Execução

1. Clone o repositório:
```bash
git clone https://github.com/seu-usuario/guga-fs.git
cd guga-fs
```

2. Execute o terminal interativo:
```bash
go run .
```

3. Executar script de testes automatizados (PowerShell / CMD):

- **PowerShell:**
  ```powershell
  Get-Content demo_etapa2.txt | go run .
  ```

- **CMD (Prompt de Comando):**
  ```cmd
  go run main.go < demo_etapa2.txt
  ```

4. Demonstração da Etapa 3 (login, ACL, criptografia, antivírus; as senhas vêm nas linhas seguintes ao comando):

  ```powershell
  Get-Content demo_etapa3.txt | go run .
  ```

> Ao iniciar, é preciso autenticar: `login --user root` (senha `root123`).


## Exemplos de Uso

### 1. Escalonamento de Disco (SSTF) e Escrita em RAID 5
```text
guga@vfs:/$ iosched sstf
guga@vfs:/$ iosched queue add 98 183 37 122 14
guga@vfs:/$ write_file /sistema/logs/app.log "Novo Registro"

[E/S] write_file /sistema/logs/app.log
[E/S] Requisições de leitura/escrita pendentes nas trilhas: [98 183 37 122 14 12 25]
Posição atual da cabeça de leitura: Trilha 53

--- ESCALONAMENTO DE DISCO: SSTF (Shortest Seek Time First) ---
Ordem de Atendimento : 53 -> 37 -> 25 -> 14 -> 12 -> 98 -> 122 -> 183
Deslocamento da Cabeça: |53-37| + |37-25| + |25-14| + |14-12| + |12-98| + |98-122| + |122-183|
Total de Movimento    : 16 + 12 + 11 + 2 + 86 + 24 + 61 = 212 trilhas

Arranjo Ativo: RAID 5 (3 Discos Físicos, Blocos de 4 KB)
[E/S - DISTRIBUIÇÃO EM DISCO]
Faixa 0:
  Disco 1: [vazio]
  Disco 2: [Bloco 01: INODE '/sistema/logs/app.log' (4 KB)]
  Disco 3: [PARIDADE : XOR(Bloco 00, Bloco 01)]
```

### 2. Teste de Falha e Reconstrução por XOR
```text
guga@vfs:/$ write_file /dados/planilha.csv --size 8KB --fail 2

[TESTE DE FALHA SIMULADO]
- Falha forçada no Disco 2!
- Recuperando Bloco 01 via operação XOR: (Disco 1 XOR Disco 3) -> Reconstrução [OK]
- Recuperando PARIDADE da faixa 1 via operação XOR: (Disco 1 XOR Disco 3) -> Reconstrução [OK]
- Recuperando Bloco 04 via operação XOR: (Disco 1 XOR Disco 3) -> Reconstrução [OK]
- Disco 2 substituído e reconstruído por completo [OK]
```

### 3. Etapa 3 — ACL, criptografia e antivírus
```text
carlos@vfs:/home/carlos$ read_file /sistema/config.sys
[SISTEMA DE SEGURANÇA - VERIFICAÇÃO DE ACL]
Objeto      : /sistema/config.sys
Proprietário: root (Grupo: admin)
Permissões  : rwxr-x--- (750)
Ação Pedida : LEITURA por 'carlos' (Grupo: devs)

[NEGADO]: O usuário 'carlos' não possui permissão de leitura no objeto '/sistema/config.sys'.
Tentativa não autorizada registrada nos logs do sistema.

carlos@vfs:/home/carlos$ encrypt_file /home/carlos/segredo.txt --key "MinhaChave123"
Arquivo '/home/carlos/segredo.txt' criptografado com sucesso.
Conteúdo gravado nos blocos físicos: "0x8F4A12B... (Encrypted Ciphertext)"

root@vfs:/# antivirus_scan /
- /home/carlos/segredo.txt  : [OK - Criptografado e Íntegro]
- /sistema/logs/app.log     : [ALERTA] Hash do bloco difere do registro oficial!
  Resultado: Possível modificação direta não autorizada no bloco do disco.
```
(os valores do ciphertext variam: o salt é aleatório a cada criptografia)

## Tabela de Comandos Suportados

| Categoria | Comando | Sintaxe Exemplo | Descrição |
| :--- | :--- | :--- | :--- |
| **Navegação** | `cd` | `cd /sistema/logs` | Navega entre diretórios absolutos ou relativos. |
| | `ls` | `ls /sistema` | Lista arquivos e subdiretórios. |
| | `tree` | `tree` | Exibe a árvore completa do sistema de arquivos. |
| **Manipulação** | `mkdir` | `mkdir /home/user` | Cria um novo diretório. |
| | `create` | `create app.log --size 10KB` | Cria um arquivo definindo o tamanho. |
| | `write_file`| `write_file /app.log "texto" --size 8KB --fail 2` | Grava/sobrescreve arquivo e aciona teste de falha em RAID. |
| | `cat` | `cat app.log` | Lê o conteúdo do arquivo e gera requisição de E/S. |
| | `rm` | `rm /sistema/logs/app.log` | Deleta um arquivo ou diretório vazio e libera blocos. |
| | `chmod` | `chmod 755 /sistema` | Altera as permissões de acesso. |
| **Diagnóstico** | `fs_status` | `fs_status --details` | Exibe mapa de alocação de blocos e relatório de fragmentação. |
| **Escalonamento**| `iosched` | `iosched <fcfs\|sstf\|scan\|cscan>` | Altera o algoritmo de escalonamento do braço de disco. |
| | | `iosched head 53` | Define a posição inicial da cabeça de leitura. |
| | | `iosched queue add 98 183 37` | Injeta requisições externas na fila de E/S. |
| | | `iosched demo` | Executa o benchmark comparativo dos 4 algoritmos. |
| **RAID** | `raid` | `raid <0\|1\|4\|5> [--disks N]` | Reconfigura o arranjo RAID e migra os dados. |
| | | `raid status` | Exibe saúde dos discos, E/S física e estatísticas acumuladas. |
| | | `raid test N` | Simula falha forçada no Disco N e executa a rotina de reconstrução. |
| **Segurança** | `login` / `logout` / `su` / `whoami` | `login --user carlos` | Autenticação (senha lida sem eco) e sessão; mostra usuário, grupo e modo. |
| | `sudo` | `sudo raid 5` | Executa 1 comando em Modo Kernel (grupo `admin`, reautentica a senha). |
| | `useradd` / `groupadd` / `usermod` / `passwd` | `useradd carlos --group devs` | Cadastro de usuários e grupos (kernel); `passwd` troca a senha com novo salt. |
| | `users` | `users --show-hash` | Lista usuários/grupos; `--show-hash` (kernel) exibe salt e hash. |
| | `chmod` / `chown` | `chmod 750 /sistema` · `chown root:admin /sistema` | Permissões rwx (dono ou kernel) e troca de dono/grupo (kernel). |
| | `setfacl` / `getfacl` | `setfacl -m u:maria:r-- arq` | ACL estendida por usuário/grupo (`-m`, `-x`, `-b`). |
| | `ls -l` | `ls -l /sistema` | Lista com dono, grupo e permissões. |
| | `read_file` | `read_file /sistema/config.sys` | Igual ao `cat`, com verificação de ACL. |
| | `encrypt_file` / `decrypt_file` | `encrypt_file arq --key "Chave"` | Criptografia AES-256-CTR em repouso. `cat arq --key "Chave"` decifra na leitura. |
| | `antivirus_scan` | `antivirus_scan / [--repair]` | Auditoria de hashes por bloco e assinaturas; `--repair` (kernel) restaura via RAID/quarentena. |
| | `disk_tamper` | `disk_tamper /sistema/logs/app.log` | (kernel) Simula escrita direta no bloco físico. |
| | `audit_log` | `audit_log 20` | (kernel) Últimos eventos do log de auditoria. |
| **Geral** | `clear` | `clear` | Limpa a tela do terminal. |
| | `exit` | `exit` | Encerra o simulador. |