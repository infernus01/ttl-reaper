/*
Copyright 2024 The Knative Authors

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

package main

import (
	"knative.dev/pkg/injection/sharedmain"

	"github.com/infernus01/knative-demo/pkg/reconciler/ttlreaper"

	_ "github.com/infernus01/knative-demo/pkg/client/injection/informers/clusterops/v1alpha1/ttlreaper"
	_ "github.com/infernus01/knative-demo/pkg/client/injection/informers/factory"
	_ "knative.dev/pkg/client/injection/kube/client"
	_ "knative.dev/pkg/injection/clients/dynamicclient"
)

func main() {
	sharedmain.Main("ttlreaper-controller", ttlreaper.NewController)
}
