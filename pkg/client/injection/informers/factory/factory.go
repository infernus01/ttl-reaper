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

package factory

import (
	"context"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"

	versioned "github.com/infernus01/knative-demo/pkg/generated/clientset/versioned"
	informers "github.com/infernus01/knative-demo/pkg/generated/informers/externalversions"
)

func init() {
	injection.Default.RegisterInformerFactory(withInformerFactory)
}

// Key is used as the key for associating information with a context.Context.
type Key struct{}

func withInformerFactory(ctx context.Context) context.Context {
	c, err := rest.InClusterConfig()
	if err != nil {
		logging.FromContext(ctx).Fatalw("Failed to get in cluster config", "error", err)
	}
	return context.WithValue(ctx, Key{},
		informers.NewSharedInformerFactory(versioned.NewForConfigOrDie(c), controller.GetResyncPeriod(ctx)))
}

// Get extracts the InformerFactory from the context.
func Get(ctx context.Context) informers.SharedInformerFactory {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		logging.FromContext(ctx).Fatal("Unable to fetch informers.SharedInformerFactory from context.")
	}
	return untyped.(informers.SharedInformerFactory)
}
