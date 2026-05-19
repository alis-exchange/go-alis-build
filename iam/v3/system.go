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
// Admin identities bypass authorization checks but keep their original
// identity type for resource names, policy members, and audit trails.
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

func (i *Identity) IsPrivileged() bool {
	return i.IsSystem() || i.IsAdmin()
}

var SystemIdentity = &Identity{
	Type: System,
	ID:   "system",
}
