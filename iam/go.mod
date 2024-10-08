module go.alis.build/iam

go 1.23.1

require (
	cloud.google.com/go/iam v1.2.1
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/google/uuid v1.6.0
	go.alis.build/alog v0.0.19
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.34.2
	open.alis.services/protobuf v1.92.0
)

require (
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	google.golang.org/genproto v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
)

// export GOPROXY=https://europe-west1-go.pkg.dev/alis-org-777777/openprotos-go,https://proxy.golang.org,direct && export GONOSUMDB=open.alis.services && go mod tidy
