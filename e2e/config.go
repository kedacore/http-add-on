package main

type config struct {
	Namespace          string `envconfig:"NAMESPACE"`
	AddonChartLocation string `envconfig:"ADD_ON_CHART_LOCATION" required:"true"`
}
