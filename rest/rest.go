// Package rest provides a simple HTTP client for REST APIs.
package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client is a simple HTTP client for REST APIs.
type Client struct {
	client  *http.Client
	baseURI string
}

// NewClient creates a new REST Client at the given base URI.
func NewClient(client *http.Client, baseURI string) *Client {
	return &Client{
		client:  client,
		baseURI: baseURI,
	}
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newError(code int, message string, args ...any) *Error {
	if len(args) == 0 {
		message = fmt.Sprintf(message, args...)
	}
	return &Error{
		Code:    code,
		Message: message,
	}
}

func (e *Error) Error() string {
	if e.Code == 0 {
		return e.Message
	}
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

// Get makes a GET request to the given path and unmarshals the response into the given response object.
//   - path: the path to make the request to
//   - response: a pointer to the struct to unmarshal the response into
func (c *Client) Get(path string, response any) *Error {
	return c.DoWithoutBody("GET", path, response)
}

// Delete makes a DELETE request to the given path and unmarshals the response into the given response object.
//   - path: the path to make the request to
//   - response: a pointer to the struct to unmarshal the response into
func (c *Client) Delete(path string, response any) *Error {
	return c.DoWithoutBody("DELETE", path, response)
}

// DoWithoutBody makes a request to the given path and unmarshals the response into the given response object.
//   - method: the HTTP method to use (e.g. GET, POST, PUT, DELETE)
//   - path: the path to make the request to
//   - response: a pointer to the struct to unmarshal the response into
func (c *Client) DoWithoutBody(method, path string, response any) *Error {
	// create http request
	httpReq, err := http.NewRequest(method, c.baseURI+path, nil)
	if err != nil {
		return newError(0, "creating http request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// make the request and unmarshal the response
	return c.Do(httpReq, response)
}

// Post makes a POST request to the given path with the JSON encoding of the given body.
// It also unmarshals the JSON response into the given response object.
// Automatically sets the Content-Type header to application/json.
func (c *Client) Post(path string, body any, response any) *Error {
	return c.DoWithBody("POST", path, body, response)
}

// Put makes a PUT request to the given path with the JSON encoding of the given body.
// It also unmarshals the JSON response into the given response object.
// Automatically sets the Content-Type header to application/json.
func (c *Client) Put(path string, body any, response any) *Error {
	return c.DoWithBody("PUT", path, body, response)
}

// Patch makes a PATCH request to the given path with the JSON encoding of the given body.
// It also unmarshals the JSON response into the given response object.
// Automatically sets the Content-Type header to application/json.
func (c *Client) Patch(path string, body any, response any) *Error {
	return c.DoWithBody("PATCH", path, body, response)
}

// DoWithBody makes a request to the given path with the JSON encoding of the given body.
// It also unmarshals the JSON response into the given response object.
// Automatically sets the Content-Type header to application/json.
//   - method: the HTTP method to use (e.g. GET, POST, PUT, DELETE)
//   - path: the path to make the request to
//   - body: the body to send with the request
//   - response: a pointer to the struct to unmarshal the response into
func (c *Client) DoWithBody(method, path string, body any, response any) *Error {
	// marshal the request body
	jsonData, err := json.Marshal(body)
	if err != nil {
		return newError(0, "marshalling request body: %v", err)
	}

	// create http request
	httpReq, err := http.NewRequest(method, c.baseURI+path, bytes.NewBuffer(jsonData))
	if err != nil {
		return newError(0, "creating http request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// make the request and unmarshal the response
	return c.Do(httpReq, response)
}

func (c *Client) Do(req *http.Request, response any) *Error {
	// make the request
	resp, err := c.client.Do(req)
	if err != nil {
		return newError(0, "making request: %v", err)
	}
	defer resp.Body.Close()

	// parse the response body
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return newError(0, "reading response body: %v", err)
	}

	// check for error status codes
	if resp.StatusCode < 200 || resp.StatusCode > 300 {
		resp.Body.Close()
		return newError(resp.StatusCode, string(respBytes))
	}

	// parse response
	err = json.Unmarshal(respBytes, response)
	if err != nil {
		return newError(0, "unmarshalling response: %v", err)
	}
	return nil
}
