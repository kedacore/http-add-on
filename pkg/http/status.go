package http

// StatusClientClosedRequest is the nginx-originated convention (HTTP 499) for
// a request the client cancelled before the response was ready. It is not a
// status code registered with IANA, so it is not defined in net/http.
const StatusClientClosedRequest = 499
