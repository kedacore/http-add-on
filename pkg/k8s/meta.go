package k8s

import "fmt"

func labels(name string) map[string]string {
	return map[string]string{
		"name": name,
		"app":  fmt.Sprintf("cscaler-%s", name),
	}
}
