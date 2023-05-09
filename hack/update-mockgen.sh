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

OUTPUT="$(mktemp -d)"

GEN='operator/generated'
CPY='hack/boilerplate.go.txt'
PKG='mock'

MOCKGEN_PKG="${MOCKGEN_PKG:-$(go list -f '{{ .Dir }}' -m github.com/golang/mock 2>/dev/null)/mockgen}"
MOCKGEN="${OUTPUT}/mockgen"
go build -o "${MOCKGEN}" "${MOCKGEN_PKG}"

for SRC in $(find "${GEN}" -type 'f' -name '*.go' | grep -v '/fake/' | grep -v "/${PKG}/")
do
  DIR="$(dirname "${SRC}")/${PKG}"
  mkdir -p "${DIR}"
  DST="${DIR}/$(basename "${SRC}")"
  "${MOCKGEN}" -copyright_file="${CPY}" -destination="${DST}" -package="${PKG}" -source="${SRC}"
done

rm -fR "${OUTPUT}"
