package k8s

import "fmt"

type NameNamespaceInfo struct {
	Name string
	Namespace string
}

func Labels(name string) map[string]string {
	return map[string]string{
		"name": name,
		"app":  fmt.Sprintf("kedahttp-%s", name),
	}
}
