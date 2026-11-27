/*
Copyright 2026 The KEDA Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"os"
	"strings"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apivalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	schemacel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

func TestRoutingRuleHostValidation(t *testing.T) {
	tests := []struct {
		name  string
		hosts []string
		valid bool
	}{
		{"omitted", nil, true},
		{"empty list", []string{}, true},
		{"empty catch-all", []string{""}, true},
		{"wildcard catch-all", []string{"*"}, true},
		{"catch-all with port", []string{"*:8080"}, true},
		{"single label", []string{"localhost"}, true},
		{"DNS", []string{"example.com"}, true},
		{"mixed case", []string{"API.Example.COM:443"}, true},
		{"hyphen", []string{"my-api.example.com"}, true},
		{"trailing dot", []string{"api.example.com.:8080"}, true},
		{"wildcard", []string{"*.example.com"}, true},
		{"wildcard with port", []string{"*.Example.COM:8080"}, true},
		{"IPv4", []string{"192.0.2.1"}, true},
		{"IPv4 with port", []string{"192.0.2.1:8080"}, true},
		{"IPv6", []string{"2001:db8::1"}, true},
		{"IPv6 uppercase", []string{"2001:DB8::ABCD"}, true},
		{"IPv6 full", []string{"2001:db8:0:1:2:3:4:5"}, true},
		{"IPv6 unspecified", []string{"::"}, true},
		{"IPv6 loopback", []string{"::1"}, true},
		{"bracketed IPv6", []string{"[2001:db8::1]"}, true},
		{"IPv6 with port", []string{"[2001:db8::1]:8080"}, true},
		{"IPv6 embedded IPv4", []string{"[64:ff9b::192.0.2.1]:443"}, true},
		{"zero port", []string{"example.com:0"}, true},
		{"maximum port", []string{"[::1]:65535"}, true},
		{"leading zero port", []string{"example.com:00080"}, true},
		{"zero-padded port", []string{"example.com:000080"}, true},
		{"maximum label", []string{strings.Repeat("a", 63) + ".com"}, true},
		{"maximum host", []string{strings.Repeat("a.", 125) + "com"}, true},
		{"maximum entries", []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}, true},
		{"empty brackets", []string{"[]"}, false},
		{"empty brackets with port", []string{"[]:80"}, false},
		{"bracketed DNS", []string{"[example.com]"}, false},
		{"bracketed IPv4", []string{"[192.0.2.1]:80"}, false},
		{"unclosed IPv6", []string{"[::1"}, false},
		{"extra bracket", []string{"[::1]]:80"}, false},
		{"IPv6 too many groups", []string{"1:2:3:4:5:6:7:8:9"}, false},
		{"IPv6 invalid hex", []string{"[gggg::1]:80"}, false},
		{"IPv6 multiple compression", []string{"2001::db8::1"}, false},
		{"IPv6 zone", []string{"[fe80::1%eth0]:80"}, false},
		{"IPv4-mapped IPv6", []string{"::ffff:192.0.2.1"}, false},
		{"scheme", []string{"https://example.com"}, false},
		{"path", []string{"example.com/path"}, false},
		{"query", []string{"example.com?foo=bar"}, false},
		{"fragment", []string{"example.com#foo"}, false},
		{"userinfo", []string{"user@example.com"}, false},
		{"space", []string{" example.com"}, false},
		{"newline", []string{"example.com\n"}, false},
		{"underscore", []string{"api_server.example.com"}, false},
		{"empty label", []string{"api..example.com"}, false},
		{"leading dot", []string{".example.com"}, false},
		{"leading hyphen", []string{"-api.example.com"}, false},
		{"trailing hyphen", []string{"api-.example.com"}, false},
		{"long label", []string{strings.Repeat("a", 64) + ".com"}, false},
		{"partial wildcard", []string{"api*.example.com"}, false},
		{"multiple wildcards", []string{"*.*.example.com"}, false},
		{"wildcard without suffix", []string{"*."}, false},
		{"port without host", []string{":80"}, false},
		{"empty port", []string{"example.com:"}, false},
		{"IPv6 empty port", []string{"[::1]:"}, false},
		{"named port", []string{"example.com:http"}, false},
		{"negative port", []string{"example.com:-1"}, false},
		{"out of range port", []string{"example.com:65536"}, false},
		{"IPv6 out of range port", []string{"[::1]:65536"}, false},
		{"long port", []string{"example.com:100000"}, false},
		{"long host", []string{strings.Repeat("a.", 126) + "com"}, false},
		{"too many entries", []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q"}, false},
		{"invalid among valid", []string{"example.com", "[]", "*.example.org"}, false},
	}

	crd := loadInterceptorRouteCRD(t)
	var internal apiextensions.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, &internal, nil); err != nil {
		t.Fatal(err)
	}
	structural, err := schema.NewStructural(&internal)
	if err != nil {
		t.Fatal(err)
	}
	openapiValidator, _, err := validation.NewSchemaValidator(&internal)
	if err != nil {
		t.Fatal(err)
	}
	celValidator := schemacel.NewValidator(structural, true, celconfig.PerCallLimit)
	for _, location := range []string{"rules", "staticRoutes"} {
		t.Run(location, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					rule := map[string]any{}
					if tt.hosts != nil {
						hosts := make([]any, len(tt.hosts))
						for i, host := range tt.hosts {
							hosts[i] = host
						}
						rule["hosts"] = hosts
					}
					rules := []any{rule}
					spec := map[string]any{
						"target":        map[string]any{"service": "test-service", "port": int64(8080)},
						"scalingMetric": map[string]any{"concurrency": map[string]any{"targetValue": int64(1)}},
					}
					if location == "staticRoutes" {
						spec[location] = []any{map[string]any{"rules": rules, "response": map[string]any{"statusCode": int64(200)}}}
					} else {
						spec[location] = rules
					}
					obj := map[string]any{"apiVersion": "http.keda.sh/v1beta1", "kind": "InterceptorRoute", "metadata": map[string]any{"name": "test"}, "spec": spec}
					errs := validation.ValidateCustomResource(field.NewPath("root"), obj, openapiValidator)
					if celValidator != nil {
						celErrs, _ := celValidator.Validate(t.Context(), field.NewPath("root"), structural, obj, nil, celconfig.RuntimeCELCostBudget)
						errs = append(errs, celErrs...)
					}
					if tt.valid && len(errs) != 0 {
						t.Fatalf("expected hosts %q to be accepted, got %v", tt.hosts, errs)
					}
					if !tt.valid && len(errs) == 0 {
						t.Fatalf("expected hosts %q to be rejected", tt.hosts)
					}
				})
			}
		})
	}
}

func TestInterceptorRouteCRDValidation(t *testing.T) {
	crd := loadInterceptorRouteCRD(t)
	var internal apiextensions.CustomResourceDefinition
	if err := apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, &internal, nil); err != nil {
		t.Fatal(err)
	}
	// The API server populates storedVersions before validating a new CRD.
	for _, v := range internal.Spec.Versions {
		if v.Storage {
			internal.Status.StoredVersions = append(internal.Status.StoredVersions, v.Name)
		}
	}
	// Also check estimated CEL cost, which evaluating individual rules does not enforce.
	if errs := apivalidation.ValidateCustomResourceDefinition(t.Context(), &internal); len(errs) != 0 {
		t.Fatalf("generated CRD cannot be installed: %v", errs)
	}
}

func loadInterceptorRouteCRD(t *testing.T) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	data, err := os.ReadFile("../../../../config/crd/bases/http.keda.sh_interceptorroutes.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.UnmarshalStrict(data, &crd); err != nil {
		t.Fatal(err)
	}
	return &crd
}
