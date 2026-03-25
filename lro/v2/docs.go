/*
Package lro provides a client for managing long-running operations stored in Spanner
and resumed via Cloud Tasks.

The v2 API replaces package-global initialization with an explicit client:

	client, err := lro.New("launchpad-v1", mux)
	if err != nil {
		return err
	}

	op, err := client.NewOperation(ctx, "operations/123", metadata)

Use [WithHost] to override the default Cloud Run host that is inferred from
`ALIS_RUN_HASH`.
*/
package lro // import "go.alis.build/lro/v2"
