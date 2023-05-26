package http

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeHostsWithOnlyHosts(t *testing.T) {
	r := require.New(t)

	testInfra := newCommonTestInfra("testns", "testapp")
	spec := testInfra.httpso.Spec

	r.NoError(sanitizeHosts(
		testInfra.logger,
		&testInfra.httpso,
	))

	r.Equal(spec.Hosts, testInfra.httpso.Spec.Hosts)
	r.Nil(testInfra.httpso.Spec.Host)
}

func TestSanitizeHostsWithBothHostAndHosts(t *testing.T) {
	r := require.New(t)

	testInfra := newBrokenTestInfra("testns", "testapp")

	err := sanitizeHosts(
		testInfra.logger,
		&testInfra.httpso,
	)
	r.Error(err)
}

func TestSanitizeHostsWithOnlyHost(t *testing.T) {
	r := require.New(t)

	testInfra := newDeprecatedTestInfra("testns", "testapp")
	spec := testInfra.httpso.Spec

	r.NoError(sanitizeHosts(
		testInfra.logger,
		&testInfra.httpso,
	))

	r.NotEqual(spec.Hosts, testInfra.httpso.Spec.Hosts)
	r.NotEqual(spec.Host, testInfra.httpso.Spec.Host)
	r.Nil(testInfra.httpso.Spec.Host)
	r.Equal([]string{*spec.Host}, testInfra.httpso.Spec.Hosts)
}

func TestSanitizeHostsWithNoHostOrHosts(t *testing.T) {
	r := require.New(t)

	testInfra := newEmptyHostTestInfra("testns", "testapp")

	err := sanitizeHosts(
		testInfra.logger,
		&testInfra.httpso,
	)
	r.Error(err)
	r.Nil(testInfra.httpso.Spec.Host)
	r.Nil(testInfra.httpso.Spec.Hosts)
}
