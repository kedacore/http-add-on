package e2e

import "strings"

type config struct {
	Namespace               string `envconfig:"NAMESPACE"`
	RunSetupTeardown        bool   `envconfig:"RUN_SETUP_TEARDOWN" default:"false"`
	AddonChartLocation      string `envconfig:"ADD_ON_CHART_LOCATION" required:"true"`
	ExampleAppChartLocation string `envconfig:"EXAMPLE_APP_CHART_LOCATION" required:"true"`
	OperatorImg             string `envconfig:"KEDAHTTP_OPERATOR_IMAGE"`
	InterceptorImg          string `envconfig:"KEDAHTTP_INTERCEPTOR_IMAGE"`
	ScalerImg               string `envconfig:"KEDAHTTP_SCALER_IMAGE"`
	HTTPAddOnImageTag       string `envconfig:"KEDAHTTP_IMAGE_TAG"`
	NumReqsAgainstProxy     int    `envconfig:"NUM_REQUESTS_TO_EXECUTE" default:"10000"`
}

func (c *config) httpAddOnHelmVars() map[string]string {
	ret := map[string]string{}
	if c.OperatorImg != "" {
		ret["images.operator"] = strings.Split(
			c.OperatorImg,
			":",
		)[0]
	}
	if c.InterceptorImg != "" {
		ret["images.interceptor"] = strings.Split(
			c.InterceptorImg,
			":",
		)[0]
	}
	if c.ScalerImg != "" {
		ret["images.scaler"] = strings.Split(
			c.ScalerImg,
			":",
		)[0]
	}
	if c.HTTPAddOnImageTag != "" {
		ret["images.tag"] = c.HTTPAddOnImageTag
	}
	return ret
}
