package authz

import (
	"context"
	"strings"

	"go.alis.build/authz/internal/jwt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func getAllRolePermissions(rolesMap map[string]*Role, role string) []string {
	perms := rolesMap[role].Permissions
	for _, extendedRole := range rolesMap[role].Extends {
		perms = append(perms, getAllRolePermissions(rolesMap, extendedRole)...)
	}
	return perms
}

func getAuthInfoWithoutRoles(ctx context.Context, superAdmins []string) (*AuthInfo, error) {
	// first get the current principal from the auth header that cloudrun used to do Authentication on the request
	authInfo, err := getAuthInfoWithoutRolesFromJwtHeader(ctx, ServerlessAuthHeader1, superAdmins, true)
	if err != nil {
		authInfo, err = getAuthInfoWithoutRolesFromJwtHeader(ctx, ServerlessAuthHeader2, superAdmins, true)
	}
	if err != nil {
		authInfo, err = getAuthInfoWithoutRolesFromJwtHeader(ctx, AuthorizationHeader, superAdmins, true)
	}
	if err != nil {
		authInfo, err = getAuthInfoWithoutRolesFromJwtHeader(ctx, AuthorizationHeader2, superAdmins, true)
	}

	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
	}

	// if authInfo is a service account ending on "@gcp-sa-iap.iam.gserviceaccount.com", trust IAPJWTAssertionHeader
	if authInfo.IsServiceAccount && strings.HasSuffix(authInfo.Email, "@gcp-sa-iap.iam.gserviceaccount.com") {
		authInfo, err = getAuthInfoWithoutRolesFromJwtHeader(ctx, IAPJWTAssertionHeader, superAdmins, false)
		if err != nil {
			return nil, err
		}
		return authInfo, nil
	}

	// if a valid principal was found in the authorization header and the principal is a super admin, look in the auth forwarding header
	// for any forwarded authorization and if not found, return the principal from the authorization header
	if authInfo.IsSuperAdmin {
		forwardedAuthInfo, err := getAuthInfoWithoutRolesFromJwtHeader(ctx, AuthForwardingHeader, superAdmins, true)
		if err == nil {
			return forwardedAuthInfo, nil
		}
	}

	return authInfo, nil
}

func getAuthInfoWithoutRolesFromJwtHeader(ctx context.Context, header string, superAdmins []string, allowTitledHeader bool) (*AuthInfo, error) {
	authInfo := &AuthInfo{}

	// Retrieve the metadata from the context.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
	}
	if len(md.Get(header)) == 0 && allowTitledHeader {
		header = strings.ToUpper(header[:1]) + header[1:]
	}

	if len(md.Get(header)) > 0 {
		// Get token from header
		token := strings.TrimPrefix(md.Get(header)[0], "Bearer ")
		token = strings.TrimPrefix(token, "bearer ")

		// Using our internal library, parse the token and extract the payload.
		payload, err := jwt.ParsePayload(token)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "%s", err)
		}

		// TODO: remove signature in case hit was directly to cloudrun (iso via consumers gateway/IAP) using "authorization" i.s.o. "x-serverless-authorization" header
		subjectParts := strings.Split(payload.Subject, ":")
		id := subjectParts[0]
		if len(subjectParts) > 1 {
			id = subjectParts[1]
		}
		authInfo.Jwt = token
		authInfo.Id = id
		authInfo.Email = payload.Email
		authInfo.IsServiceAccount = strings.HasSuffix(payload.Email, ".gserviceaccount.com")

		if authInfo.IsServiceAccount {
			authInfo.PolicyMember = "serviceAccount:" + authInfo.Id
		} else {
			authInfo.PolicyMember = "user:" + authInfo.Id
		}
		authInfo.IsSuperAdmin = sliceContains(superAdmins, authInfo.PolicyMember, authInfo.Email)
		return authInfo, nil

	} else {
		return nil, status.Error(codes.Unauthenticated, "unable to retrieve metadata from the request header")
	}
}

func sliceContains(stringSlice []string, search1 string, search2 string) bool {
	for _, s := range stringSlice {
		if s == search1 || s == search2 {
			return true
		}
	}
	return false
}
