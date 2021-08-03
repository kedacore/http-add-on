package e2e

type config struct {
	Namespace               string `envconfig:"NAMESPACE"`
	AddonChartLocation      string `envconfig:"ADD_ON_CHART_LOCATION" required:"true"`
	ExampleAppChartLocation string `envconfig:"EXAMPLE_APP_CHART_LOCATION" required:"true"`
	OperatorImg             string `envconfig:"KEDAHTTP_OPERATOR_IMAGE"`
	InterceptorImg          string `envconfig:"KEDAHTTP_INTERCEPTOR_IMAGE"`
	ScalerImg               string `envconfig:"KEDAHTTP_SCALER_IMAGE"`
	HTTPAddOnImageTag       string `envconfig:"KEDAHTTP_IMAGE_TAG"`
}

func (c *config) httpAddOnHelmVars() map[string]string {
	ret := map[string]string{}
	if c.OperatorImg != "" {
		ret["images.operator"] = c.OperatorImg
	}
	if c.InterceptorImg != "" {
		ret["images.interceptor"] = c.InterceptorImg
	}
	if c.ScalerImg != "" {
		ret["images.scaler"] = c.ScalerImg
	}
	if c.HTTPAddOnImageTag != "" {
		ret["images.tag"] = c.HTTPAddOnImageTag
	}
	return ret
}
