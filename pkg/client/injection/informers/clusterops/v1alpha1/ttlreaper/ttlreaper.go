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

package ttlreaper

import (
	"context"

	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"

	factory "github.com/infernus01/knative-demo/pkg/client/injection/informers/factory"
	ttlreaperv1alpha1 "github.com/infernus01/knative-demo/pkg/generated/informers/externalversions/clusterops/v1alpha1"
)

func init() {
	injection.Default.RegisterInformer(withInformer)
}

// Key is used as the key for associating information with a context.Context.
type Key struct{}

func withInformer(ctx context.Context) (context.Context, controller.Informer) {
	f := factory.Get(ctx)
	inf := f.Clusterops().V1alpha1().TTLReapers()
	return context.WithValue(ctx, Key{}, inf), inf.Informer()
}

// Get extracts the typed informer from the context.
func Get(ctx context.Context) ttlreaperv1alpha1.TTLReaperInformer {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		logging.FromContext(ctx).Fatal("Unable to fetch ttlreaperv1alpha1.TTLReaperInformer from context.")
	}
	return untyped.(ttlreaperv1alpha1.TTLReaperInformer)
}
