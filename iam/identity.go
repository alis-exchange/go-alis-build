package iam

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"go.alis.build/iam/intenal/jwt"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const (
	// One of the headers that cloudrun uses to send the JWT token of the authorized requester
	AuthHeader = "authorization"
	// One of the headers that cloudrun uses to send the JWT token of the authorized requester
	ServerlessAuthHeader = "x-serverless-authorization"
	// The header that this Alis Build package uses to forward the JWT token of the authorized requester
	AlisForwardingHeader = "x-alis-forwarded-authorization"
	// The header that Google Cloud ESPv2 proxy uses to forward the JWT token of the authorized requester
	ProxyForwardingHeader = "x-forwarded-authorization"
	// The header that Google Cloud IAP uses to forward the JWT token of the authorized requester
	IAPJWTAssertionHeader = "x-goog-iap-jwt-assertion"
)

// Identity represents details on the entiry making the particular rpc request.
type Identity struct {
	// The original jwt token
	jwt string
	// The requester id e.g. 123456789, m-0000-0000-9245-2134
	id string
	// The requester email e.g. john@gmail.com or alis-build@...gserviceaccount.com
	email string
	// Whether the requester is the deployment service account
	isDeploymentServiceAccount bool
	// The Policy on the User resource of the requester.
	// Not applicable for service accounts
	policy *iampb.Policy
}

func (r *Identity) Id() string {
	return r.id
}

func (r *Identity) Email() string {
	return r.email
}

func (r *Identity) Jwt() string {
	return r.jwt
}

func (r *Identity) IsDeploymentServiceAccount() bool {
	return r.isDeploymentServiceAccount
}

func (r *Identity) Policy() *iampb.Policy {
	return r.policy
}

// Returns whether the requester used a google identity to authenticate.
func (r *Identity) IsGoogleIdentity() bool {
	// if first char of id is a number, it is a google identity
	return '0' <= r.id[0] && r.id[0] <= '9'
}

// Returns whether the requester is a service account.
func (r *Identity) IsServiceAccount() bool {
	return strings.HasSuffix(r.email, "@gserviceaccount.com")
}

// Returns the policy member string of the requester.
// E.g. user:123456789 or serviceAccount:alis-build@...
func (r *Identity) PolicyMember() string {
	if r.IsServiceAccount() {
		return "serviceAccount:" + r.email
	} else {
		return "user:" + r.id
	}
}

// Returns the user name of the requester.
// Format: users/{userId}
func (r *Identity) UserName() string {
	return "users/" + r.id
}

// ExtractIdentityFromCtx returns the Identity making the request or for whom the request is being forwarded by a super admin.
func ExtractIdentityFromCtx(ctx context.Context, deploymentServiceAccountEmail string) (*Identity, error) {
	// Looks in the specified header for a JWT token and extracts the requester from it.
	// Returns nil if no JWT token was found in the header.
	// Returns an error if the JWT token is invalid.
	getIdentityFromJwtHeader := func(ctx context.Context, header string, deploymentServiceAccountEmail string) (*Identity, error) {
		identity := &Identity{}

		// Retrieve the metadata from the context.
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, fmt.Errorf("retrieve metadata from context")
		}

		if len(md.Get(header)) > 0 {
			// Get token from header
			token := strings.TrimPrefix(md.Get(header)[0], "Bearer ")
			token = strings.TrimPrefix(token, "bearer ")

			// Extract token payload
			payload, err := jwt.ParsePayload(token)
			if err != nil {
				return nil, fmt.Errorf("parse jwt payload: %w", err)
			}

			identity.jwt = token
			identity.id = payload.Subject
			identity.email = payload.Email

			// If the Identity is not a service account, see if we could extract the iam Policy object.
			if !identity.IsServiceAccount() {
				// policy is base64 encoded version of the bytes of the policy
				policyString, ok := payload.Claims["policy"].(string)
				if ok && payload.Issuer == "alis.build" {
					policyBytes, err := base64.StdEncoding.DecodeString(policyString)
					if err != nil {
						return nil, fmt.Errorf("decode jwt iam policy: %w", err)
					}
					if len(policyBytes) > 0 {
						policy := &iampb.Policy{}
						err = proto.Unmarshal(policyBytes, policy)
						if err != nil {
							return nil, fmt.Errorf("unmarshal jwt iam policy: %w", err)
						}
						identity.policy = policy
					}
				}
			}
			identity.isDeploymentServiceAccount = identity.email == deploymentServiceAccountEmail
			return identity, nil

		} else {
			return nil, fmt.Errorf("unable to extract Identity from jwt token")
		}
	}

	// first try to get the principal from the main AuthHeader
	identity, err := getIdentityFromJwtHeader(ctx, AuthHeader, deploymentServiceAccountEmail)
	if err != nil {
		// if no header is found in the AuthHeader, try to get the Identity from the Google Cloud Serverless header
		identity, err = getIdentityFromJwtHeader(ctx, ServerlessAuthHeader, deploymentServiceAccountEmail)
		if err != nil {
			// if no principal is found, we assume the the Identity is a super admin
			// This is most likely the scenario where hits are between internal methods.
			identity = &Identity{
				email:                      deploymentServiceAccountEmail,
				isDeploymentServiceAccount: true,
			}
		}
	}

	// if principal is a service account ending on "@gcp-sa-iap.iam.gserviceaccount.com", trust IAPJWTAssertionHeader
	if identity.IsServiceAccount() && strings.HasSuffix(identity.email, "@gcp-sa-iap.iam.gserviceaccount.com") {
		identity, err = getIdentityFromJwtHeader(ctx, IAPJWTAssertionHeader, deploymentServiceAccountEmail)
		if err != nil {
			return nil, fmt.Errorf("retrieve forwarded principal from the IAP request header: %w", err)
		}
		return identity, nil
	}

	// if principal is the Deployment Service Account, check for forwarded principal
	if identity.IsDeploymentServiceAccount() {
		for _, header := range []string{AlisForwardingHeader, ProxyForwardingHeader} {
			forwaredIdentity, err := getIdentityFromJwtHeader(ctx, header, deploymentServiceAccountEmail)
			if err != nil {
				continue
			} else {
				// Use the forwarded identity as the identity.
				identity = forwaredIdentity
				break
			}
		}
	}

	return identity, nil
}
