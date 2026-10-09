package iam

import (
	"strings"
	"time"
)

// MemberCreditLimitReached is the CreditStanding reason for a user blocked by
// their own spend limit, as opposed to the account's.
const MemberCreditLimitReached = "MEMBER_CREDIT_LIMIT_REACHED"

// CreditStanding is what the identity's token says about charging
// credit-costing work to an account. Mirrors go.alis.build/iam/v3.
type CreditStanding struct {
	// False when the token cannot answer: no standing claim, or the account is
	// not the active account. Callers decide what unknown means.
	Known   bool
	Allowed bool
	// An alis.os.accounts.v1 AllowTransactionResponse.Reason name, or
	// MEMBER_CREDIT_LIMIT_REACHED. Empty when allowed.
	Reason string
	// Safe to show the end user. Empty when allowed.
	Message string
	// When a member's own block lapses. Zero unless Reason is
	// MEMBER_CREDIT_LIMIT_REACHED.
	BlockedUntil time.Time
}

// CreditStanding reports whether the token allows credit-costing work to be
// charged to account ("accounts/{id}" or the bare id) at now. The token only
// describes the active account, so any other account is unknown. An account
// block outranks the user's own spend block.
func (r *Identity) CreditStanding(account string, now time.Time) CreditStanding {
	if r == nil || r.activeAccount == nil || r.activeAccount.AccountStanding == nil ||
		strings.TrimPrefix(account, "accounts/") != r.ActiveAccountID() {
		return CreditStanding{}
	}
	if s := r.activeAccount.AccountStanding; !s.Allow {
		return CreditStanding{Known: true, Reason: s.Reason, Message: creditMessage(s.Reason)}
	}
	if until := r.activeAccount.MemberBlockedUntil; until != nil {
		if t := time.Unix(until.Seconds, int64(until.Nanos)).UTC(); now.Before(t) {
			return CreditStanding{
				Known:        true,
				Reason:       MemberCreditLimitReached,
				Message:      creditMessage(MemberCreditLimitReached),
				BlockedUntil: t,
			}
		}
	}
	return CreditStanding{Known: true, Allowed: true}
}

// creditMessage is the user-facing text for a standing reason. Keep in step
// with go.alis.build/iam/v3 and with ledger.Verdict in alis.os.accounts.v1,
// which returns the same text from AllowTransaction.
func creditMessage(reason string) string {
	switch reason {
	case "NO_BILLING_DETAILS":
		return "Please add your billing details to continue."
	case "TRIAL_EXPIRED":
		return "Your account trial is expired. Please upgrade your plan to continue."
	case "ACCOUNT_ARCHIVED":
		return "This account is archived."
	case MemberCreditLimitReached:
		return "You have reached your personal credit limit for this billing period. Please contact your account administrator to adjust your limit."
	default: // ACCOUNT_CREDIT_LIMIT_REACHED, and any reason added later
		return "Your account has reached its credit limit. Please purchase new credits or upgrade your plan to continue."
	}
}
