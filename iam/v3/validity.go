package iam

import "strings"

// Validity is whether the identity's user may do credit-costing work on an
// account, as alis.os.accounts.v1 decided it (Member.validity) when the token
// was minted.
type Validity struct {
	Valid bool
	// Why not, as an alis.os.accounts.v1 Member.Validity.Reason name, e.g.
	// "NO_BILLING_DETAILS": use it to choose what to offer the user. Valid
	// decides, not Reason: a name you don't know still means not valid. Empty
	// when valid.
	Reason string
	// Why not in words, safe to show the user as it is. Empty when valid.
	Comments string
}

// Validity reports the user's validity on account ("accounts/{id}" or the
// bare id). ok is false when the token can't answer: the token only describes
// the active account, and carries no validity when accounts.v1 hasn't decided
// one for the member. Callers then ask alis.os.accounts.v1 AllowTransaction.
func (i *Identity) Validity(account string) (v Validity, ok bool) {
	if i == nil || i.ActiveAccount == nil || i.ActiveAccount.Valid == nil ||
		strings.TrimPrefix(account, "accounts/") != i.ActiveAccountID() {
		return Validity{}, false
	}
	a := i.ActiveAccount
	return Validity{Valid: *a.Valid, Reason: a.Reason, Comments: a.Comments}, true
}
