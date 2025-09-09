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
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	ttlreaperinformer "github.com/infernus01/knative-demo/pkg/client/injection/informers/clusterops/v1alpha1/ttlreaper"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	dynamicclient "knative.dev/pkg/injection/clients/dynamicclient"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "ttlreaper-controller"
)

// NewController creates a Reconciler and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	ttlreaperInformer := ttlreaperinformer.Get(ctx)

	c := &Reconciler{
		kubeclientset:   kubeclient.Get(ctx),
		dynamicClient:   dynamicclient.Get(ctx),
		ttlreaperLister: ttlreaperInformer.Lister(),
		timers:          make(map[string]*time.Timer),
	}

	impl := controller.NewContext(ctx, c, controller.ControllerOptions{
		WorkQueueName: controllerAgentName,
		Logger:        logger,
	})

	logger.Info("Setting up event handlers")

	// Set up an event handler for when TTLReaper resources change
	ttlreaperInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Start watching for target resources dynamically based on TTLReaper specs
	go c.watchTargetResources(ctx, impl)

	return impl
}

// watchTargetResources dynamically watches ALL resource types that TTLReapers target
func (c *Reconciler) watchTargetResources(ctx context.Context, impl *controller.Impl) {
	logger := logging.FromContext(ctx)
	watchedGVRs := make(map[schema.GroupVersionResource]bool)

	// Check every 30 seconds for new TTLReaper configurations
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ttlreapers, err := c.ttlreaperLister.List(labels.Everything())
			if err != nil {
				logger.Errorw("Failed to list TTLReapers for dynamic watching", "error", err)
				continue
			}

			// Collect all unique GVRs that TTLReapers are targeting
			targetGVRs := make(map[schema.GroupVersionResource][]string)
			for _, ttlreaper := range ttlreapers {
				gvr, err := c.parseTargetGVR(ttlreaper.Spec.TargetKind, ttlreaper.Spec.TargetAPIVersion)
				if err != nil {
					logger.Errorw("Failed to parse target GVR", "error", err, "ttlreaper", ttlreaper.Name)
					continue
				}

				key := fmt.Sprintf("%s/%s", ttlreaper.Spec.TargetKind, ttlreaper.Spec.TargetAPIVersion)
				targetGVRs[gvr] = append(targetGVRs[gvr], key)
			}

			// Start watching any new GVRs
			for gvr, targetSpecs := range targetGVRs {
				if !watchedGVRs[gvr] {
					c.startWatchingGVR(ctx, impl, gvr, targetSpecs[0])
					watchedGVRs[gvr] = true
					logger.Infow("Started watching resource type", "gvr", gvr.String())
				}
			}
		}
	}
}

// startWatchingGVR creates a dynamic informer for the given GVR
func (c *Reconciler) startWatchingGVR(ctx context.Context, impl *controller.Impl, gvr schema.GroupVersionResource, targetSpec string) {
	dynamicInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return c.dynamicClient.Resource(gvr).Watch(ctx, metav1.ListOptions{})
			},
		},
		&unstructured.Unstructured{},
		controller.GetResyncPeriod(ctx),
		cache.Indexers{},
	)

	// When target resource changes, enqueue all TTLReapers that target it
	dynamicInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if u, ok := obj.(*unstructured.Unstructured); ok {
				c.enqueueTargetingTTLReapers(ctx, impl, u.GetKind(), u.GetAPIVersion())
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if u, ok := newObj.(*unstructured.Unstructured); ok {
				c.enqueueTargetingTTLReapers(ctx, impl, u.GetKind(), u.GetAPIVersion())
			}
		},
	})

	// Start the dynamic informer
	go dynamicInformer.Run(ctx.Done())
}

// enqueueTargetingTTLReapers finds all TTLReapers that target the given kind/apiVersion and enqueues them
func (c *Reconciler) enqueueTargetingTTLReapers(ctx context.Context, impl *controller.Impl, targetKind, targetAPIVersion string) {
	logger := logging.FromContext(ctx)

	ttlreapers, err := c.ttlreaperLister.List(labels.Everything())
	if err != nil {
		logger.Errorw("Failed to list TTLReapers", "error", err)
		return
	}

	for _, ttlreaper := range ttlreapers {
		if ttlreaper.Spec.TargetKind == targetKind && ttlreaper.Spec.TargetAPIVersion == targetAPIVersion {
			impl.Enqueue(ttlreaper)
		}
	}
}

// parseTargetGVR converts targetKind and targetAPIVersion to GroupVersionResource
func (c *Reconciler) parseTargetGVR(targetKind, targetAPIVersion string) (schema.GroupVersionResource, error) {
	// Parse API version (e.g., "workflows.example.com/v1" -> group="workflows.example.com", version="v1")
	parts := strings.Split(targetAPIVersion, "/")
	if len(parts) != 2 {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid targetAPIVersion format: %s", targetAPIVersion)
	}

	group := parts[0]
	version := parts[1]

	// Convert Kind to plural resource name (basic pluralization)
	resource := strings.ToLower(targetKind) + "s"

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}, nil
}
