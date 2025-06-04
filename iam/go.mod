module github.com/alis-exchange/iam

go 1.24.2

require (
	cloud.google.com/go/iam v1.5.2
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/google/uuid v1.6.0
	go.alis.build/alog v0.0.19
	google.golang.org/grpc v1.72.2
	google.golang.org/protobuf v1.36.6
)

require (
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto v0.0.0-20250303144028-a0af3efb3deb // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250414145226-207652e42e2e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250414145226-207652e42e2e // indirect
)

// export GOPROXY=https://europe-west1-go.pkg.dev/alis-org-777777/openprotos-go,https://proxy.golang.org,direct && export GONOSUMDB=open.alis.services && go mod tidy
