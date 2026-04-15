package authz

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"go.alis.build/iam/v3"
)

func isMember(identity *iam.Identity, members []string) bool {
	for _, memberText := range members {
		member := new(Member).parse(memberText)
		resolver, ok := memberResolvers[member.Type]
		if ok {
			if resolver(identity, member) {
				return true
			}
		}
	}
	return false
}

type Member struct {
	Type string
	ID   string
}

func (m *Member) parse(text string) *Member {
	parts := strings.Split(text, ":")
	m.Type = parts[0]
	if len(parts) > 1 {
		m.ID = strings.Join(parts[1:], ":")
	}
	return m
}

var (
	memberResolversMu sync.RWMutex
	memberResolvers   = map[string]func(identity *iam.Identity, member *Member) bool{
		"user": func(identity *iam.Identity, member *Member) bool {
			return identity.Type == iam.User && identity.ID == member.ID
		},
		"serviceAccount": func(identity *iam.Identity, member *Member) bool {
			return identity.Type == iam.ServiceAccount && identity.ID == member.ID
		},
		"domain": func(identity *iam.Identity, member *Member) bool {
			return strings.HasSuffix(identity.Email, "@"+member.ID)
		},
		"group": func(identity *iam.Identity, member *Member) bool {
			return slices.Contains(identity.GroupIDs, member.ID)
		},
		"email": func(identity *iam.Identity, member *Member) bool {
			return identity.Email == member.ID
		},
	}
)

func AddMemberResolver(memberTypes []string, resolver func(identity *iam.Identity, member *Member) bool) error {
	memberResolversMu.Lock()
	defer memberResolversMu.Unlock()

	for _, memberType := range memberTypes {
		if _, ok := memberResolvers[memberType]; ok {
			return fmt.Errorf("resolver already registered for '%s'", memberType)
		}
		memberResolvers[memberType] = resolver
	}
	return nil
}
