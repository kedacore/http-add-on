package http

// StatusClientClosedRequest is the nginx-originated convention (HTTP 499) for
// a request the client cancelled before the response was ready. It is not a
// status code registered with IANA, so it is not defined in net/http.
const StatusClientClosedRequest = 499

// StatusTextClientClosedRequest is the status text for StatusClientClosedRequest.
// http.StatusText doesn't know about 499 since it isn't an IANA-registered code.
const StatusTextClientClosedRequest = "Client Closed Request"
