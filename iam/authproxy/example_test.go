package authproxy_test

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/alis-exchange/iam/authproxy"
)

func ExampleHandleAuth() {
	// Create a new AuthProxy instance
	authHost := "https://iam-auth-" + os.Getenv("ALIS_RUN_HASH") + ".run.app"
	authProxy := authproxy.New(authHost)

	// Define your HTTP handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Let authproxy handle authentication
		if authProxy.HandleAuth(w, r) {
			return // Request handled by authproxy
		}

		// Your application logic here...
		fmt.Fprintln(w, "Hello, authenticated user!")
	})

	// Start the server
	fmt.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
