// Package protobundle builds the proto bundle a Spanner database needs
// before a table can declare PROTO or ENUM columns: the list of type names
// for CREATE PROTO BUNDLE, and the FileDescriptorSet bytes that travel
// alongside the DDL.
//
// A Bundle is built from root messages with New, or from message and enum
// descriptors with NewFromDescriptors, which also takes a top-level enum
// that only an ENUM column uses. It follows every message- and enum-typed
// field from the roots, recursively, so nested types such as
// google.spanner.admin.database.v1.Backup.State are never forgotten; adds
// every message containing a nested type it lists, as Spanner requires;
// and skips the synthetic map-entry messages Spanner does not accept as
// types. The descriptors come from the roots themselves, normally the
// registry linked into the binary, so the schema always matches the Go
// types and no .proto files or protoc are needed. Lookup returns a bundled
// descriptor by name.
//
//	bundle, err := protobundle.New(&iampb.Policy{})
//	if err != nil {
//		return err
//	}
//	descriptors, err := bundle.Descriptors()
//	if err != nil {
//		return err
//	}
//	op, err := admin.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
//		Parent:          instance,
//		CreateStatement: "CREATE DATABASE `library`",
//		ExtraStatements: []string{
//			bundle.CreateStatement(),
//			"CREATE TABLE Shelves (`key` STRING(MAX) NOT NULL, Policy `google.iam.v1.Policy`) PRIMARY KEY (`key`)",
//		},
//		ProtoDescriptors: descriptors,
//	})
//
// For tests against the Spanner emulator, spannertest.NewDatabase does all
// of this in one call.
package protobundle
