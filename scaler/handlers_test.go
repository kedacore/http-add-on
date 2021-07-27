package main

import (
	context "context"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	externalscaler "github.com/kedacore/http-add-on/proto"
	"github.com/stretchr/testify/require"
)

func TestIsActive(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()
	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()

	hdl := newImpl(
		lggr,
		pinger,
	)
	res, err := hdl.IsActive(
		ctx,
		&externalscaler.ScaledObjectRef{},
	)
	r.NoError(err)
	r.True(res.Result)
}

func TestGetMetricSpec(t *testing.T) {
	const (
		host   = "abcd"
		target = int64(200)
	)
	r := require.New(t)
	ctx := context.Background()
	lggr := logr.Discard()
	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()
	hdl := newImpl(lggr, pinger)
	meta := map[string]string{
		"host":   host,
		"target": strconv.Itoa(int(target)),
	}
	ref := &externalscaler.ScaledObjectRef{
		ScalerMetadata: meta,
	}
	ret, err := hdl.GetMetricSpec(ctx, ref)
	r.NoError(err)
	r.Equal(1, len(ret.MetricSpecs))
	spec := ret.MetricSpecs[0]
	r.Equal(host, spec.MetricName)
	r.Equal(target, spec.TargetSize)
}

func TestGetMetrics(t *testing.T) {
	t.Fatal("TODO")
}
