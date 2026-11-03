// Package spending enforces authoritative spending decisions. It does not
// calculate balances, grant resource access, or choose a payer for the caller.
package spending

import (
	"fmt"
	"regexp"
)

// Kind distinguishes member-attributed work from explicitly account-owned work.
type Kind uint8

const (
	Unspecified Kind = iota
	Member
	Account
)

// Payer is selected and authorized by the producing service, then persisted
// before execution. A worker's service identity is never the financial payer.
type Payer struct {
	Kind    Kind
	Account string
	User    string
}

func MemberPayer(account, user string) Payer {
	return Payer{Kind: Member, Account: account, User: user}
}

func AccountPayer(account string) Payer { return Payer{Kind: Account, Account: account} }

var (
	accountName = regexp.MustCompile(`^accounts/[A-Za-z0-9_-]{1,128}$`)
	userName    = regexp.MustCompile(`^users/[A-Za-z0-9_-]{1,128}$`)
)

func (p Payer) Validate() error {
	if !accountName.MatchString(p.Account) {
		return fmt.Errorf("a valid paying account is required")
	}
	switch p.Kind {
	case Member:
		if !userName.MatchString(p.User) {
			return fmt.Errorf("member spending requires a valid billed user")
		}
	case Account:
		if p.User != "" {
			return fmt.Errorf("account-owned spending cannot contain a billed user")
		}
	default:
		return fmt.Errorf("an explicit payer kind is required")
	}
	return nil
}
