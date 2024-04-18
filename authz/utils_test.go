package authz

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func Test_getAuthInfoWithoutRoles(t *testing.T) {
	type args struct {
		ctx         context.Context
		superAdmins []string
	}
	playIcBearer := "Bearer eyJhbGciOiJSUzI1NiIsImtpZCI6ImFkZjVlNzEwZWRmZWJlY2JlZmE5YTYxNDk1NjU0ZDAzYzBiOGVkZjgiLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiJodHRwczovL3Jlc291cmNlcy1tYXBzLXYxLWRtZXFsYngzcmEtZXcuYS5ydW4uYXBwIiwiYXpwIjoiYWxpcy1idWlsZEBwbGF5LWljLWRldi1sZ3AuaWFtLmdzZXJ2aWNlYWNjb3VudC5jb20iLCJlbWFpbCI6ImFsaXMtYnVpbGRAcGxheS1pYy1kZXYtbGdwLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImV4cCI6MTcxMTYxNDgwMCwiaWF0IjoxNzExNjExMjAwLCJpc3MiOiJodHRwczovL2FjY291bnRzLmdvb2dsZS5jb20iLCJzdWIiOiIxMDM3MjA4Mjg4ODEyOTg4NzIyODgifQ.SIGNATURE_REMOVED_FOR_TESTING"
	playMcBearer := "bearer eyJhbGciOiJSUzI1NiIsImtpZCI6ImFkZjVlNzEwZWRmZWJlY2JlZmE5YTYxNDk1NjU0ZDAzYzBiOGVkZjgiLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiIzMjU1NTk0MDU1OS5hcHBzLmdvb2dsZXVzZXJjb250ZW50LmNvbSIsImF6cCI6ImFsaXMtYnVpbGRAcGxheS1tYy1kZXYtNHBlLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwiZW1haWwiOiJhbGlzLWJ1aWxkQHBsYXktbWMtZGV2LTRwZS5pYW0uZ3NlcnZpY2VhY2NvdW50LmNvbSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlLCJleHAiOjE3MTE2MTYzNDcsImlhdCI6MTcxMTYxMjc0NywiaXNzIjoiaHR0cHM6Ly9hY2NvdW50cy5nb29nbGUuY29tIiwic3ViIjoiMTA5NzY0Njc5NzYyMjIxOTIwMzk0In0.SIGNATURE_REMOVED_FOR_TESTING"

	directAuthCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", playIcBearer))
	espv2ForwardedAuthCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", playIcBearer, "x-forwarded-authorization", playMcBearer))

	osIapServiceAccountBearer := "bearer eyJhbGciOiJSUzI1NiIsImtpZCI6IjkzYjQ5NTE2MmFmMGM4N2NjN2E1MTY4NjI5NDA5NzA0MGRhZjNiNDMiLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiIvcHJvamVjdHMvNTk3Njk2Nzg2MzE2L2xvY2F0aW9ucy9ldXJvcGUtd2VzdDEvc2VydmljZXMvc2VydmljZXMtaWRlbnRpdHktdjEiLCJhenAiOiIxMDg2OTQ2ODY5NDE4ODg3MjA4NDciLCJlbWFpbCI6InNlcnZpY2UtNTk3Njk2Nzg2MzE2QGdjcC1zYS1pYXAuaWFtLmdzZXJ2aWNlYWNjb3VudC5jb20iLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwiZXhwIjoxNzEyNTY1Mzk3LCJpYXQiOjE3MTI1NjE3OTcsImlzcyI6Imh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbSIsInN1YiI6IjEwODY5NDY4Njk0MTg4ODcyMDg0NyJ9.SIGNATURE_REMOVED_FOR_TESTING"
	userViaIAPBearer := "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6InZaSWJRQSJ9.eyJhdWQiOiIvcHJvamVjdHMvNTk3Njk2Nzg2MzE2L2dsb2JhbC9iYWNrZW5kU2VydmljZXMvMzg0NTA4MDQ1MjgyNzU0NDQ3OCIsImF6cCI6Ii9wcm9qZWN0cy81OTc2OTY3ODYzMTYvZ2xvYmFsL2JhY2tlbmRTZXJ2aWNlcy8zODQ1MDgwNDUyODI3NTQ0NDc4IiwiZW1haWwiOiJwZXRlci5tYnVpQGFsaXN4LmNvbSIsImV4cCI6MTcxMjU2MjM5NywiaGQiOiJhbGlzeC5jb20iLCJpYXQiOjE3MTI1NjE3OTcsImlkZW50aXR5X3NvdXJjZSI6IkdPT0dMRSIsImlzcyI6Imh0dHBzOi8vY2xvdWQuZ29vZ2xlLmNvbS9pYXAiLCJzdWIiOiJhY2NvdW50cy5nb29nbGUuY29tOjEwMTkxMzEwNDM4Nzc5MjA0NjQyNiJ9.SIGNATURE_REMOVED_FOR_TESTING"
	iapAuthCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-serverless-authorization", osIapServiceAccountBearer, "x-goog-iap-jwt-assertion", userViaIAPBearer))

	tests := []struct {
		name    string
		args    args
		want    *AuthInfo
		wantErr bool
	}{
		{
			name: "DirectAuth",
			args: args{
				ctx:         directAuthCtx,
				superAdmins: []string{"serviceAccount:103720828881298872288"},
			},
		},
		{
			name: "Espv2ForwardedAuth",
			args: args{
				ctx:         espv2ForwardedAuthCtx,
				superAdmins: []string{"serviceAccount:103720828881298872288"},
			},
		},
		{
			name: "IapAuth",
			args: args{
				ctx:         iapAuthCtx,
				superAdmins: []string{"serviceAccount:103720828881298872288"},
			},
		},
		{
			name: "EmptyCtx",
			args: args{
				ctx:         context.Background(),
				superAdmins: []string{"serviceAccount:103720828881298872288"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getAuthInfoWithoutRoles(tt.args.ctx, tt.args.superAdmins)
			if (err != nil) != tt.wantErr {
				t.Errorf("getAuthInfoWithoutRoles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Logf("got: %+v", got)
		})
	}
}
