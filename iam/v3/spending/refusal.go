package spending

import (
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Domain marks the ErrorInfo of a refusal to do chargeable work.
const Domain = "alis.os.accounts"

// legacyDomain is what the Ideas service attaches. It shares the domain with
// unrelated reasons, so there a refusal is recognised by its account.
const legacyDomain = "alis.build"

// Refusal says why a payer may not incur cost. Reason is an open set (account
// standing names, their accounts.v1 forerunners): a reader that does not know
// one treats it as blocked and shows Comments.
type Refusal struct {
	Reason   string
	Account  string // "accounts/{id}", when the producer named it
	Comments string
}

// Refused is the error every producer returns for a refusal. The code and the
// "REASON: comments" message are kept for callers that still read those.
func Refused(reason, account, comments string) error {
	if comments == "" {
		comments = "This payer does not currently allow chargeable work."
	}
	info := &errdetails.ErrorInfo{Reason: reason, Domain: Domain}
	if account != "" {
		info.Metadata = map[string]string{"account": account}
	}
	s, err := status.New(codes.PermissionDenied, reason+": "+comments).WithDetails(info)
	if err != nil {
		return status.Error(codes.PermissionDenied, comments)
	}
	return s.Err()
}

// FromError reports whether err is a refusal, whatever its gRPC code. Read it
// before mapping the code: a refusal is not a rate limit or a missing role.
func FromError(err error) (Refusal, bool) {
	s, ok := status.FromError(err)
	if !ok || err == nil {
		return Refusal{}, false
	}
	for _, detail := range s.Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok || info.GetReason() == "" {
			continue
		}
		account := info.GetMetadata()["account"]
		if info.GetDomain() != Domain && (info.GetDomain() != legacyDomain || !strings.HasPrefix(account, "accounts/")) {
			continue
		}
		// A caller may have wrapped the error, which puts its own words and
		// "rpc error: ..." in front of the message.
		comments := s.Message()
		if _, after, found := strings.Cut(comments, info.GetReason()+": "); found {
			comments = after
		}
		return Refusal{Reason: info.GetReason(), Account: account, Comments: comments}, true
	}
	return Refusal{}, false
}
