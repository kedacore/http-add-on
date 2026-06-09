package http

import nethttp "net/http"

// NewTransports returns two transports cloned from base, split by protocol.
// A single transport cannot do both HTTP/1 and h2c on plain http:// URLs:
// the transport only uses h2c when UnencryptedHTTP2 is set AND HTTP1 is not.
//
//   - defaultTransport: HTTP/1 for plain HTTP, ALPN-negotiated for TLS.
//   - http2OnlyTransport: h2c for plain HTTP, h2 for TLS.
func NewTransports(base *nethttp.Transport) (*nethttp.Transport, *nethttp.Transport) {
	var defaultProto nethttp.Protocols
	defaultProto.SetHTTP1(true)
	defaultProto.SetHTTP2(true) // enables ALPN negotiation for TLS backends

	var http2OnlyProto nethttp.Protocols
	http2OnlyProto.SetHTTP2(true)
	http2OnlyProto.SetUnencryptedHTTP2(true) // with HTTP1 unset, this forces h2c for plain HTTP

	defaultTransport := base.Clone()
	defaultTransport.Protocols = &defaultProto

	http2OnlyTransport := base.Clone()
	http2OnlyTransport.Protocols = &http2OnlyProto

	return defaultTransport, http2OnlyTransport
}
