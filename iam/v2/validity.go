package iam

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

// CreditValidity reports whether the user may do credit-costing work on their
// active account, which is the account such work is always charged to. ok is
// false when the token can't answer, because accounts.v1 hasn't decided a
// validity for the member. Callers then ask alis.os.accounts.v1
// AllowTransaction.
func (r *Identity) CreditValidity() (v Validity, ok bool) {
	if r == nil || r.activeAccount == nil || r.activeAccount.Valid == nil {
		return Validity{}, false
	}
	a := r.activeAccount
	return Validity{Valid: *a.Valid, Reason: a.Reason, Comments: a.Comments}, true
}
