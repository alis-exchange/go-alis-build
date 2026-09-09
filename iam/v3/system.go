package iam

import (
	"os"
	"slices"
)

var (
	systemEmails = []string{}
	adminEmails  = []string{}
)

func init() {
	alisOsProjectEnv := os.Getenv("ALIS_OS_PROJECT")
	if alisOsProjectEnv != "" {
		environmentServiceAccountEmail := "alis-build@" + alisOsProjectEnv + ".iam.gserviceaccount.com"
		systemEmails = append(systemEmails, environmentServiceAccountEmail)
	}
}

// AddSystemEmail registers email as a trusted system identity.
//
// It is intended for process startup configuration, typically from init
// functions, and must not be called concurrently with identity checks.
func AddSystemEmail(email string) {
	systemEmails = append(systemEmails, email)
}

// AddAdminEmail registers email as a privileged admin identity.
//
// Admin identities bypass authorization checks, unless the credential is
// Restricted, but keep their original identity type for resource names, policy
// members, and audit trails.
//
// It is intended for process startup configuration, typically from init
// functions, and must not be called concurrently with identity checks.
func AddAdminEmail(email string) {
	adminEmails = append(adminEmails, email)
}

func (i *Identity) checkIfSystem() {
	if slices.Contains(systemEmails, i.Email) {
		i.Type = System
	}
}

func (i *Identity) IsSystem() bool {
	return i.Type == System
}

func (i *Identity) IsAdmin() bool {
	return slices.Contains(adminEmails, i.Email)
}

// IsPrivileged reports whether this credential bypasses authz role and
// permission checks. It is true for system and admin identities unless the
// credential is Restricted.
//
// Bypass decisions must use IsPrivileged rather than IsSystem or IsAdmin, so a
// restricted credential can never exceed the roles it explicitly carries.
func (i *Identity) IsPrivileged() bool {
	return (i.IsSystem() || i.IsAdmin()) && !i.Restricted
}

var SystemIdentity = &Identity{
	Type: System,
	ID:   "system",
}
