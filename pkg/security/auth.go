package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)


type Mode int

const (
	ModeUser Mode = iota
	ModeKernel
)

func (m Mode) String() string {
	if m == ModeKernel {
		return "MODO_KERNEL"
	}
	return "MODO_USUARIO"
}

const (
	RootUser            = "root"
	AdminGroup          = "admin"
	DefaultRootPassword = "root123"
	hashIterations      = 10000
)

type User struct {
	Name   string
	UID    int
	Group  string
	Groups []string
	Salt   string
	Hash   string
}

type Group struct {
	Name string
	GID  int
}

type LogEntry struct {
	Time    time.Time
	User    string
	Event   string
	Allowed bool
	Detail  string
}

type System struct {
	Users   map[string]*User
	Groups  map[string]*Group
	Current *User
	Mode    Mode
	Audit   []LogEntry

	Hashes map[int]string

	ReadSecret func(prompt string) string

	nextUID int
	nextGID int
}

func NewSystem() *System {
	s := &System{
		Users:   map[string]*User{},
		Groups:  map[string]*Group{},
		Hashes:  map[int]string{},
		nextUID: 1000,
		nextGID: 1000,
	}
	s.Groups[AdminGroup] = &Group{Name: AdminGroup, GID: 0}
	s.Groups["users"] = &Group{Name: "users", GID: 100}
	s.Groups["devs"] = &Group{Name: "devs", GID: 101}

	salt := NewSalt()
	s.Users[RootUser] = &User{
		Name:  RootUser,
		UID:   0,
		Group: AdminGroup,
		Salt:  salt,
		Hash:  HashPassword(DefaultRootPassword, salt),
	}
	return s
}

// ---------- Hash de senhas ----------

func NewSalt() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("falha ao gerar salt: %v", err))
	}
	return hex.EncodeToString(b)
}

func HashPassword(password, salt string) string {
	data := []byte(salt + password)
	sum := sha256.Sum256(data)
	for i := 1; i < hashIterations; i++ {
		buf := make([]byte, 0, len(sum)+len(data))
		buf = append(buf, sum[:]...)
		buf = append(buf, data...)
		sum = sha256.Sum256(buf)
	}
	return hex.EncodeToString(sum[:])
}

func SecureEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func ShortHash(h string) string {
	if len(h) <= 16 {
		return h
	}
	return h[:16] + "..."
}

// ---------- Cadastro ----------

func validName(name string) bool {
	if name == "" || len(name) > 32 {
		return false
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.'
		if !ok {
			return false
		}
	}
	return true
}

func (s *System) AddGroup(name string) error {
	if !validName(name) {
		return fmt.Errorf("nome de grupo inválido '%s'", name)
	}
	if _, exists := s.Groups[name]; exists {
		return fmt.Errorf("o grupo '%s' já existe", name)
	}
	s.Groups[name] = &Group{Name: name, GID: s.nextGID}
	s.nextGID++
	return nil
}

func (s *System) AddUser(name, group, password string) (*User, error) {
	if !validName(name) {
		return nil, fmt.Errorf("nome de usuário inválido '%s'", name)
	}
	if _, exists := s.Users[name]; exists {
		return nil, fmt.Errorf("o usuário '%s' já existe", name)
	}
	if _, ok := s.Groups[group]; !ok {
		return nil, fmt.Errorf("o grupo '%s' não existe (use groupadd)", group)
	}
	if len(password) < 4 {
		return nil, fmt.Errorf("senha muito curta (mínimo 4 caracteres)")
	}
	salt := NewSalt()
	u := &User{
		Name:  name,
		UID:   s.nextUID,
		Group: group,
		Salt:  salt,
		Hash:  HashPassword(password, salt),
	}
	s.nextUID++
	s.Users[name] = u
	return u, nil
}

func (s *System) AddToGroup(user, group string) error {
	u, ok := s.Users[user]
	if !ok {
		return fmt.Errorf("usuário '%s' não existe", user)
	}
	if _, ok := s.Groups[group]; !ok {
		return fmt.Errorf("grupo '%s' não existe", group)
	}
	if s.IsMember(u, group) {
		return fmt.Errorf("'%s' já pertence ao grupo '%s'", user, group)
	}
	u.Groups = append(u.Groups, group)
	return nil
}

func (s *System) SetPassword(u *User, password string) error {
	if len(password) < 4 {
		return fmt.Errorf("senha muito curta (mínimo 4 caracteres)")
	}
	u.Salt = NewSalt()
	u.Hash = HashPassword(password, u.Salt)
	return nil
}

func (s *System) IsMember(u *User, group string) bool {
	if u == nil {
		return false
	}
	if u.Group == group {
		return true
	}
	for _, g := range u.Groups {
		if g == group {
			return true
		}
	}
	return false
}

func (s *System) SortedUsers() []*User {
	list := make([]*User, 0, len(s.Users))
	for _, u := range s.Users {
		list = append(list, u)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].UID < list[j].UID })
	return list
}

func (s *System) SortedGroups() []*Group {
	list := make([]*Group, 0, len(s.Groups))
	for _, g := range s.Groups {
		list = append(list, g)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].GID < list[j].GID })
	return list
}

// ---------- Autenticação e sessão ----------

func (s *System) Authenticate(name, password string) (*User, bool) {
	u, ok := s.Users[name]
	if !ok {
		HashPassword(password, "00000000000000000000000000000000") // custo constante
		return nil, false
	}
	if SecureEqual(HashPassword(password, u.Salt), u.Hash) {
		return u, true
	}
	return nil, false
}

func (s *System) Login(u *User) {
	s.Current = u
	if u.UID == 0 {
		s.Mode = ModeKernel
	} else {
		s.Mode = ModeUser
	}
}

func (s *System) Logout() {
	s.Current = nil
	s.Mode = ModeUser
}

func (s *System) CurrentName() string {
	if s == nil || s.Current == nil {
		return "(ninguém)"
	}
	return s.Current.Name
}

func (s *System) CurrentGroup() string {
	if s == nil || s.Current == nil {
		return "-"
	}
	return s.Current.Group
}

func (s *System) CanSudo(u *User) bool {
	return u != nil && (u.UID == 0 || s.IsMember(u, AdminGroup))
}

// ---------- Modos de execução ----------

func (s *System) IsKernel() bool {
	return s == nil || s.Mode == ModeKernel
}

// RequireKernel é a "chamada de sistema" que protege operações críticas.
func (s *System) RequireKernel(op string) bool {
	if s.IsKernel() {
		return true
	}
	fmt.Printf("\n[MODO KERNEL REQUERIDO] A operação '%s' é crítica e exige privilégios elevados.\n", op)
	fmt.Printf("O usuário '%s' está em %s. Use 'sudo <comando>' (membros do grupo '%s') ou autentique-se como root.\n",
		s.CurrentName(), s.Mode, AdminGroup)
	fmt.Println("Tentativa não autorizada registrada nos logs do sistema.")
	s.Record("PRIVILEGIO_NEGADO", false, op)
	return false
}

// WithKernel executa fn em modo kernel (identidade root) e depois restaura a sessão.
func (s *System) WithKernel(fn func()) {
	prevUser, prevMode := s.Current, s.Mode
	s.Current = s.Users[RootUser]
	s.Mode = ModeKernel
	defer func() {
		s.Current, s.Mode = prevUser, prevMode
	}()
	fn()
}

// ---------- Auditoria ----------

func (s *System) Record(event string, allowed bool, detail string) {
	if s == nil {
		return
	}
	s.Audit = append(s.Audit, LogEntry{
		Time:    time.Now(),
		User:    s.CurrentName(),
		Event:   event,
		Allowed: allowed,
		Detail:  detail,
	})
}
