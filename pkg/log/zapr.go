package log

import (
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
)

func NewZapr() (logr.Logger, error) {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Sampling = &zap.SamplingConfig{
		Initial:    1,
		Thereafter: 5,
	}
	zapLggr, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	return zapr.NewLogger(zapLggr), nil
}
