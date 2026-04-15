test-all:
	make test-auth
	make test-authz
	make test-authn
	
test-auth:
	go test -v .
test-authz:
	go test -v ./authz
test-authn:
	go test -v ./authn
