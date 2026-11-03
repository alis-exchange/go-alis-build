# Spending checks

`go.alis.build/iam/v3/spending` guards chargeable work using decisions from the
account service. It does not calculate credits, authorize access to resources,
choose a payer, reserve funds, or publish usage.

Persist an explicit `MemberPayer(account, user)` for interactive/personal work,
or `AccountPayer(account)` for authorized organisation-owned work. The producer
must verify resource ownership and the right to spend against that account
before persisting the payer. An anonymous participant or a worker service
identity must never implicitly become the payer. Keep the payer unchanged on
retries; a change of ownership requires a new authorized execution.

Bind typed RPC clients to `Sources.Authority`, `Sources.V2` and `Sources.V1`.
Transport, credentials and protobuf conversions stay with the consuming service
so this public package does not depend on private generated packages. Authority
responses must identify the exact account. Active accounts use v2; legacy and
shadow accounts use v1. Unknown/draining ownership and lookup failures refuse
work. No error triggers a fallback to legacy. A legacy account-only adapter
must explicitly implement that mode or return an error.

V2 readers use `BatchGetMemberStanding` for member payers and
`BatchGetAccountStanding` for account payers. Check the exact response resource,
one-result cardinality, protobuf timestamp validity and enum values before
constructing a `Decision`. Account decisions exclude personal and member-reserved
funds. Allowances need a positive revision and evaluation time; a definitive
missing-member response can deny with `MEMBER_REMOVED` without inventing a
revision. Denials need a reason. An allowance with a blocking reason is rejected.

```go
sources := spending.Sources{Authority: readAuthority, V2: readV2, V1: readV1}
guard := spending.New(sources.Read)
payer := spending.AccountPayer(savedExecution.Account)
if err := guard.Check(ctx, payer, true); err != nil {
    return err // fresh lookup when a queued/scheduled/recovered execution starts
}
// Check again immediately before every paid provider attempt, including retries.
if err := guard.Check(ctx, payer, false); err != nil {
    return err
}
```

For interactive member work, `WithToken` can carry the signature-validated JWT
authenticated by middleware, including a freshly refreshed cookie. The subject,
expiry and active account must match the authenticated identity. The token's
member verdict is usable until expiry (issuers should retain their five-minute
lifetime). Account-owned work always ignores that verdict. `current=true`
bypasses both the token and cached decision.

Remote decisions are cached for at most one minute, shortened to the next
evaluation boundary. Reads have a five-second deadline. Unavailable or malformed
responses return gRPC `Unavailable`; blocked payers return `PermissionDenied`
with an `ErrorInfo` reason. A failed refresh invalidates the previous cache entry.
Do not catch these errors and continue to the provider.

Always publish actual consumption with the saved payer and a stable event ID,
even if the payer becomes blocked after the request started. Admission is not a
reservation: concurrent/in-flight work and valid cached decisions can overshoot
a limit. Durable, idempotent usage delivery remains the producer's responsibility.

Validation: `go test -race ./spending` and `go vet ./spending` from `iam/v3`.
