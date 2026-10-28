# Simulador de Sistema de Arquivos (Virtual File System - VFS)

- **Disciplina:** Sistemas Operacionais (BCC)
- **Professor:** Douglas Veras e Silva
- **Linguagem:** Go (Golang 1.27+)


## Sobre o Projeto

Este projeto é um **Simulador de Sistema de Arquivos Virtual (VFS)** desenvolvido em **Go**, capaz de gerenciar a hierarquia lógica de diretórios e simular o mapeamento físico de blocos em disco usando **Alocação Indexada por i-node**.

O simulador conta com um terminal interativo (REPL) com suporte a cores ANSI, comandos no estilo Unix/Linux (`cd`, `ls`, `mkdir`, `create`, `cat`, `rm`, `tree`, `chmod`, `fs_status`), navegação por caminhos absolutos e relativos, e um diagnóstico detalhado do disco (análise de fragmentação interna e externa).


## Arquitetura do Código (Modular)

O projeto segue uma estrutura modular extensível em Go, separando a camada do sistema de arquivos da camada da interface de linha de comando (CLI):

```text
guga-fs/
├── main.go                       # Ponto de entrada da aplicação
├── go.mod                        # Módulo Go
└── pkg/
    ├── fs/                       # Núcleo do Sistema de Arquivos (Engine)
    │   ├── inode.go              # Estruturas de Inode, Blocos e Metadados
    │   ├── disk.go               # Inicialização, Resolução de Caminhos e Parse de Tamanhos
    │   └── allocation.go         # Operações CRUD, Árvore e Relatório de Fragmentação
    └── cli/                      # Interface do Terminal Interativo (REPL)
        ├── shell.go              # Loop principal do Terminal
        └── commands/             # Manipuladores modulares de comandos
            ├── registry.go       # Mapeamento do registro de comandos
            ├── mkdir.go          # Comando mkdir
            ├── create.go         # Comando create
            ├── cd.go             # Comando cd
            ├── ls.go             # Comando ls
            ├── cat.go            # Comando cat
            ├── rm.go             # Comando rm
            ├── tree.go           # Comando tree
            ├── fs_status.go      # Comando fs_status
            ├── chmod.go          # Comando chmod
            └── clear.go          # Comando clear

```


## Funcionalidades Implementadas

1. **Hierarquia em Árvore:**
* Navegação em diretórios com suporte a caminhos absolutos (ex.: `/sistema/logs/app.log`) e relativos (`.`, `..`).
* Visualização gráfica em árvore através do comando `tree`.


2. **Operações CRUD:**
* **Create:** `mkdir` (diretórios) e `create` (arquivos com parâmetro de tamanho ex.: `--size 10KB`).
* **Read:** `cat` (conteúdo do arquivo) e `ls` (listar diretórios).
* **Update:** Suporte a alteração de metadados e permissões com `chmod`.
* **Delete:** `rm` (remoção de arquivos e diretórios vazios com liberação automática de blocos).


3. **Mapeamento e Alocação Física (i-node):**
* Disco virtual divido em blocos de tamanho fixo (padrão: 4 KB por bloco, 16 blocos no total).
* Estratégia de **Alocação Indexada**, onde cada arquivo possui um Inode alocado que aponta para os blocos de dados.


4. **Diagnóstico e Relatório de Fragmentação:**
* **Mapa de Blocos:** Exibição da ocupação exata do disco por bloco (Inodes, Dados e Blocos Livres).
* **Fragmentação Interna:** Cálculo do desperdício de espaço dentro do último bloco ocupado por cada arquivo.
* **Fragmentação Externa:** Avaliação de blocos livres dispersos.
* **Análise de Compactação:** Indicação automática sobre a necessidade de compactar o disco.


## Como Executar

### Pré-requisitos

* **Go** (versão 1.27 ou superior) instalado na máquina.

### Passos para Execução

1. Clone o repositório:
```bash
git clone https://github.com/seu-usuario/guga-fs.git
cd guga-fs
```


2. Execute o projeto diretamente:
```bash
go run main.go
```


3. (Opcional) Compilar um binário executável:
```bash
go build -o guga-fs main.go
./guga-fs        # No Linux/macOS
guga-fs.exe      # No Windows
```



## Exemplo de Uso (Conforme Especificação da Etapa 1)

### 1. Criação e Exibição da Árvore Hierárquica

```text
guga@vfs:/$ mkdir /sistema
guga@vfs:/$ mkdir /sistema/logs
guga@vfs:/$ create /sistema/logs/app.log --size 10KB

[ÁRVORE DO SISTEMA DE ARQUIVOS]
/
└── sistema/ [DIR]
    └── logs/ [DIR]
        └── app.log (10 KB) [ARQ]

```

### 2. Alocação Indexada e Diagnóstico do Disco

```text
guga@vfs:/$ fs_status --details

Tamanho do Bloco de Disco: 4 KB | Total de Blocos: 16 (64 KB)

[MAPA DE ALOCAÇÃO DO DISCO (Indexado)]
Bloco 00 [INODE_RAIZ]    : Diretório '/'
Bloco 01 [INODE_APP]     : Arquivo '/sistema/logs/app.log'
Bloco 02 [DADO]          : '/sistema/logs/app.log' (Bloco 1 - 4096 bytes ocupados)
Bloco 03 [DADO]          : '/sistema/logs/app.log' (Bloco 2 - 4096 bytes ocupados)
Bloco 04 [DADO]          : '/sistema/logs/app.log' (Bloco 3 - 2048 bytes ocupados)
Bloco 05..15            : [LIVRE]

--- RELATÓRIO DE FRAGMENTAÇÃO ---
Tamanho Real do Arquivo 'app.log': 10240 bytes (10 KB)
Espaço Alocado no Disco          : 12288 bytes (3 blocos de 4 KB)
Fragmentação Interna             : 2048 bytes (50% de desperdício no Bloco 04)
Fragmentação Externa             : 0 bytes
Necessidade de Compactação       : NÃO (Blocos alocados via i-node indexado)

```


## Tabela de Comandos Suportados

| Comando | Sintaxe Exemplo | Descrição |
| --- | --- | --- |
| `cd` | `cd /sistema/logs` | Navega entre diretórios. |
| `ls` | `ls` ou `ls /sistema` | Lista arquivos e pastas do diretório. |
| `mkdir` | `mkdir /home/user` | Cria um novo diretório. |
| `create` | `create app.log --size 10KB` | Cria um arquivo com tamanho específico. |
| `cat` | `cat app.log` | Exibe o conteúdo de um arquivo. |
| `rm` | `rm /sistema/logs/app.log` | Deleta um arquivo ou diretório vazio. |
| `tree` | `tree` | Exibe a estrutura hierárquica completa em formato de árvore. |
| `fs_status` | `fs_status --details` | Exibe o mapa físico de blocos do disco e relatório de fragmentação. |
| `chmod` | `chmod 755 /sistema` | Altera as permissões de um arquivo/diretório. |
| `clear` | `clear` | Limpa o terminal. |
| `exit` | `exit` | Finaliza a sessão do terminal virtual. |
