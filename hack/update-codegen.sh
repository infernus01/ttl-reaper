#!/usr/bin/env bash

# Copyright 2024 The TTL Reaper Authors.
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

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

echo "🔄 Generating Kubernetes code..."

# Clean up existing generated files
echo "🧹 Cleaning up existing generated files..."
rm -f "${SCRIPT_ROOT}/pkg/apis/ttlreaper/v1alpha1/zz_generated.deepcopy.go"
rm -rf "${SCRIPT_ROOT}/pkg/client"

# Generate deepcopy methods (REQUIRED)
echo "📝 Generating deepcopy methods..."
go run k8s.io/code-generator/cmd/deepcopy-gen \
  --go-header-file="${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
  --output-file="${SCRIPT_ROOT}/pkg/apis/ttlreaper/v1alpha1/zz_generated.deepcopy.go" \
  github.com/shubbhar/ttl-reaper/pkg/apis/ttlreaper/v1alpha1

echo "✅ Essential code generation completed!"
echo ""
echo "📁 Generated files:"
echo "   ✅ pkg/apis/ttlreaper/v1alpha1/zz_generated.deepcopy.go (DeepCopy methods)"
echo ""

# Try to generate additional client code (optional - may fail with newer versions)
echo "🚀 Attempting to generate additional client code..."
echo "   (This is optional and may fail with newer k8s.io/code-generator versions)"

set +e  # Don't exit on error for optional generation

# Try to generate typed client
echo "📝 Attempting to generate typed client..."
if go run k8s.io/code-generator/cmd/client-gen \
  --go-header-file="${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
  --clientset-name="versioned" \
  --input-base="" \
  --input="github.com/shubbhar/ttl-reaper/pkg/apis/ttlreaper/v1alpha1" \
  --output-dir="${SCRIPT_ROOT}/pkg/client/clientset" \
  --output-pkg="github.com/shubbhar/ttl-reaper/pkg/client/clientset" 2>/dev/null; then
  echo "   ✅ Typed client generated"
else
  echo "   ❌ Typed client generation failed (not critical for basic controller)"
fi

set -e  # Re-enable exit on error

echo ""
echo "🎯 READY TO USE! Your controller now has properly generated Kubernetes types."
echo ""
echo "💡 The deepcopy methods are the essential part - your controller will work"
echo "   perfectly with controller-runtime using just these generated methods."
echo ""
echo "🔧 To test: run 'make build' to verify everything compiles correctly."
