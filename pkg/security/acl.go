package security

import (
	"fmt"
	"strings"
)

type Perm uint8

const (
	PermX Perm = 1
	PermW Perm = 2
	PermR Perm = 4
)

func PermName(p Perm) string {
	switch p {
	case PermR:
		return "leitura"
	case PermW:
		return "escrita"
	case PermX:
		return "execução"
	}
	return "acesso"
}

func triad(bits uint32) string {
	out := []byte("---")
	if bits&4 != 0 {
		out[0] = 'r'
	}
	if bits&2 != 0 {
		out[1] = 'w'
	}
	if bits&1 != 0 {
		out[2] = 'x'
	}
	return string(out)
}

func (p Perm) String() string { return triad(uint32(p)) }

func ModeString(mode uint32) string {
	return triad((mode>>6)&7) + triad((mode>>3)&7) + triad(mode&7)
}

type ACLEntry struct {
	Kind  string
	Name  string
	Perms Perm
}

func (e ACLEntry) String() string {
	return fmt.Sprintf("%s:%s:%s", e.Kind, e.Name, e.Perms)
}

func FormatACL(acl []ACLEntry) string {
	parts := make([]string, 0, len(acl))
	for _, e := range acl {
		parts = append(parts, e.String())
	}
	return strings.Join(parts, ", ")
}

func ParsePerms(s string) (Perm, error) {
	if len(s) == 1 && s[0] >= '0' && s[0] <= '7' {
		return Perm(s[0] - '0'), nil
	}
	var p Perm
	for _, c := range s {
		switch c {
		case 'r':
			p |= PermR
		case 'w':
			p |= PermW
		case 'x':
			p |= PermX
		case '-':
		default:
			return 0, fmt.Errorf("permissão inválida '%c' (use r, w, x ou -)", c)
		}
	}
	return p, nil
}

func ParseACLSpec(spec string, needPerms bool) (ACLEntry, error) {
	parts := strings.Split(spec, ":")
	want := 2
	if needPerms {
		want = 3
	}
	if len(parts) != want {
		return ACLEntry{}, fmt.Errorf("especificação inválida '%s' (use u:nome:rwx ou g:grupo:rwx)", spec)
	}
	var kind string
	switch parts[0] {
	case "u", "user":
		kind = "u"
	case "g", "group":
		kind = "g"
	default:
		return ACLEntry{}, fmt.Errorf("tipo '%s' inválido (use u ou g)", parts[0])
	}
	if parts[1] == "" {
		return ACLEntry{}, fmt.Errorf("nome ausente em '%s'", spec)
	}
	entry := ACLEntry{Kind: kind, Name: parts[1]}
	if needPerms {
		p, err := ParsePerms(parts[2])
		if err != nil {
			return ACLEntry{}, err
		}
		entry.Perms = p
	}
	return entry, nil
}

func SetACL(acl []ACLEntry, e ACLEntry) []ACLEntry {
	for i := range acl {
		if acl[i].Kind == e.Kind && acl[i].Name == e.Name {
			acl[i].Perms = e.Perms
			return acl
		}
	}
	return append(acl, e)
}

func RemoveACL(acl []ACLEntry, kind, name string) ([]ACLEntry, bool) {
	out := make([]ACLEntry, 0, len(acl))
	removed := false
	for _, e := range acl {
		if e.Kind == kind && e.Name == name {
			removed = true
			continue
		}
		out = append(out, e)
	}
	return out, removed
}

func (s *System) Allowed(owner, group string, mode uint32, acl []ACLEntry, want Perm) bool {
	if s.IsKernel() {
		return true
	}
	u := s.Current
	if u == nil {
		return false
	}
	w := uint32(want)

	if u.Name == owner {
		return (mode>>6)&w == w
	}
	for _, e := range acl {
		if e.Kind == "u" && e.Name == u.Name {
			return uint32(e.Perms)&w == w
		}
	}

	matched, granted := false, false
	if s.IsMember(u, group) {
		matched = true
		if (mode>>3)&w == w {
			granted = true
		}
	}
	for _, e := range acl {
		if e.Kind == "g" && s.IsMember(u, e.Name) {
			matched = true
			if uint32(e.Perms)&w == w {
				granted = true
			}
		}
	}
	if matched {
		return granted
	}
	return mode&w == w
}
