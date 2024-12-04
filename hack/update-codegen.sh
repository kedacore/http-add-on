#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
# Copyright 2023 The KEDA Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

CODEGEN_PKG="${CODEGEN_PKG:-$(go list -f '{{ .Dir }}' -m k8s.io/code-generator 2>/dev/null)}"
SCRIPT_ROOT="$(dirname "${BASH_SOURCE[0]}")/.."
OUTPUT_BASE="$(mktemp -d)"

GO_PACKAGE='github.com/kedacore/http-add-on'

source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/operator/apis"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${SCRIPT_ROOT}/operator/generated" \
    --output-pkg "github.com/kedacore/http-add-on/operator/generated" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/operator/apis"
