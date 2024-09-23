package authz

// An object that contains the role ids and the resource types where the roles could be stored in policies.
type Roles struct {
	ids           []string
	resourceTypes []string
}

// Returns the role ids that grant access to the given permission.
func (r *Roles) Ids() []string {
	return r.ids
}

// Returns the resource types where the roles could be stored in policies.
// E.g. alis.open.iam.v1.User and/or abc.de.library.v1.Book
func (r *Roles) ResourceTypes() []string {
	return r.resourceTypes
}
