package e2e

type config struct {
	Namespace               string `envconfig:"NAMESPACE"`
	AddonChartLocation      string `envconfig:"ADD_ON_CHART_LOCATION" required:"true"`
	ExampleAppChartLocation string `envconfig:"EXAMPLE_APP_CHART_LOCATION" required:"true"`
}
