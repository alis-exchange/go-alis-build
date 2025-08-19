package serviceproxy

type ConnOptions struct {
	alias          string
	allowedMethods []string
}

// ConnOption is a functional option for the AddConn and RemoveConn methods.
type ConnOption func(*ConnOptions)

// WithAlias is used to set an alias for the connection.
// This can be useful for identifying the connection in cases
// where multiple connections to the same service are made.
//
// If no alias is provided, 'default' will be used.
//
// This alias can be passed in via the x-alis-service-alias header.
// If the header is not set, the alias will be 'default'.
func WithAlias(alias string) ConnOption {
	return func(opts *ConnOptions) {
		opts.alias = alias
	}
}

// WithAllowedMethods is used to restrict the methods that can be proxied.
// For example, to allow only the method "ExampleMethod" in the service "Service" and package "org.product.v1":
//
//	WithAllowedMethods("org.product.v1.Service/ExampleMethod")
//
// To allow all methods in the service "Service" and package "org.product.v1":
//
//	WithAllowedMethods("org.product.v1.Service/*")
//
// If no methods are provided, all methods in the service will be allowed.
func WithAllowedMethods(methods ...string) ConnOption {
	return func(opts *ConnOptions) {
		opts.allowedMethods = methods
	}
}
