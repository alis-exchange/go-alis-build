package client_test

import (
	"context"
	"go.alis.build/client"
	"log"
)

func ExampleNewConn() {

	ctx := context.Background()
	conn, err := client.NewConn(ctx, "cloudrun-service.app:443", false)
	if err != nil {
		log.Println(err)
	}
	// Use the connection auto generated client packages, using the example at:
	// https://grpc.io/docs/languages/go/basics/#client, we will instantiate a client as follows:
	// 	routeGuideClient := pb.NewRouteGuideClient(conn)
	_ = conn
}
