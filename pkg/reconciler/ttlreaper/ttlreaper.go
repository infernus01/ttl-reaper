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
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	"github.com/infernus01/knative-demo/pkg/apis/clusterops/v1alpha1"
	ttlreaperlister "github.com/infernus01/knative-demo/pkg/generated/listers/clusterops/v1alpha1"
)

// Reconciler implements controller.Reconciler for TTLReaper resources.
type Reconciler struct {
	kubeclientset   kubernetes.Interface
	dynamicClient   dynamic.Interface
	ttlreaperLister ttlreaperlister.TTLReaperLister

	// Timer management for immediate TTL deletion (like Jobs)
	timers      map[string]*time.Timer
	timersMutex sync.RWMutex
}

// Check that our Reconciler implements Interface
var _ controller.Reconciler = (*Reconciler)(nil)

// Check that our Reconciler implements LeaderAware
var _ reconciler.LeaderAware = (*Reconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx).With(zap.String("ttlreaper", key))
	logger.Info("Reconciling TTLReaper")

	// Get the TTLReaper resource with this name
	ttlReaper, err := r.ttlreaperLister.Get(key)
	if errors.IsNotFound(err) {
		// The TTLReaper resource may no longer exist, in which case we stop processing.
		logger.Info("TTLReaper resource no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	return r.reconcileTTLReaper(ctx, ttlReaper)
}

func (r *Reconciler) reconcileTTLReaper(ctx context.Context, reaper *v1alpha1.TTLReaper) error {
	logger := logging.FromContext(ctx).With(zap.String("ttlreaper", reaper.Name))

	// Validate required fields
	if reaper.Spec.TargetKind == "" {
		logger.Error("TargetKind is required")
		return fmt.Errorf("targetKind is required")
	}
	if reaper.Spec.TargetAPIVersion == "" {
		logger.Error("TargetAPIVersion is required")
		return fmt.Errorf("targetAPIVersion is required")
	}

	// Parse the API version to get group and version
	gv, err := schema.ParseGroupVersion(reaper.Spec.TargetAPIVersion)
	if err != nil {
		logger.Errorw("Invalid targetAPIVersion", zap.Error(err))
		return fmt.Errorf("invalid targetAPIVersion: %w", err)
	}

	// Create the GroupVersionResource
	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: getResourceName(reaper.Spec.TargetKind), // Convert Kind to resource name
	}

	totalReaped := 0

	// Determine namespaces to process
	namespaces := []string{}
	if reaper.Spec.TargetNamespace != "" {
		namespaces = append(namespaces, reaper.Spec.TargetNamespace)
	} else {
		// List all namespaces
		nsList, err := r.kubeclientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list namespaces: %w", err)
		}
		for _, ns := range nsList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	// Process each namespace
	for _, namespace := range namespaces {
		scheduled, err := r.processNamespace(ctx, namespace, gvr, reaper.Spec.LabelSelector)
		if err != nil {
			logger.Errorw("Error processing namespace",
				zap.String("namespace", namespace),
				zap.Error(err))
			// Continue with other namespaces even if one fails
			continue
		}
		totalReaped += scheduled
	}

	logger.Infow("🎯 TTL scheduling cycle completed",
		zap.String("ttlreaper", reaper.Name),
		zap.String("targetKind", reaper.Spec.TargetKind),
		zap.String("targetAPIVersion", reaper.Spec.TargetAPIVersion),
		zap.String("targetNamespace", reaper.Spec.TargetNamespace),
		zap.Int("namespacesProcessed", len(namespaces)),
		zap.Int("totalScheduled", totalReaped))

	return nil
}

func (r *Reconciler) processNamespace(ctx context.Context, namespace string, gvr schema.GroupVersionResource, labelSelector *metav1.LabelSelector) (int, error) {
	logger := logging.FromContext(ctx).With(zap.String("namespace", namespace))

	// Build list options
	listOptions := metav1.ListOptions{}
	if labelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(labelSelector)
		if err != nil {
			return 0, fmt.Errorf("invalid label selector: %w", err)
		}
		listOptions.LabelSelector = selector.String()
	}

	// List resources of the target kind in the namespace
	resourceList, err := r.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, listOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			// Resource type doesn't exist in this cluster, skip
			logger.Debugw("Resource type not found in cluster", zap.String("gvr", gvr.String()))
			return 0, nil
		}
		return 0, fmt.Errorf("failed to list resources %s in namespace %s: %w", gvr.String(), namespace, err)
	}

	scheduled := 0
	for _, item := range resourceList.Items {
		resourceName := item.GetName()
		resourceKey := fmt.Sprintf("%s/%s/%s", namespace, item.GetKind(), resourceName)

		// Check if resource has TTL field
		ttlSeconds, hasTTL, err := unstructured.NestedInt64(item.Object, "spec", "ttlSecondsAfterFinished")
		if err != nil || !hasTTL {
			continue
		}

		// Check if resource is finished
		if !r.isResourceFinished(&item) {
			continue
		}

		// Schedule deletion at exact TTL expiration time (like Jobs)
		r.scheduleResourceDeletion(ctx, resourceKey, &item, gvr, ttlSeconds)
		scheduled++
	}

	return scheduled, nil
}

func (r *Reconciler) scheduleResourceDeletion(ctx context.Context, resourceKey string, resource *unstructured.Unstructured, gvr schema.GroupVersionResource, ttlSeconds int64) {
	logger := logging.FromContext(ctx)

	// Get completion time
	var finishTime time.Time
	completionTimeStr, found, err := unstructured.NestedString(resource.Object, "status", "completionTime")
	if found && err == nil {
		if parsedTime, parseErr := time.Parse(time.RFC3339, completionTimeStr); parseErr == nil {
			finishTime = parsedTime
		}
	}

	// Fallback to creation time if no completion time
	if finishTime.IsZero() {
		finishTime = resource.GetCreationTimestamp().Time
	}

	// Calculate exact expiration time
	ttlDuration := time.Duration(ttlSeconds) * time.Second
	expirationTime := finishTime.Add(ttlDuration)

	// Calculate delay until expiration
	delay := time.Until(expirationTime)

	// Cancel existing timer if any
	r.timersMutex.Lock()
	if existingTimer, exists := r.timers[resourceKey]; exists {
		existingTimer.Stop()
		delete(r.timers, resourceKey)
	}

	// If already expired, delete immediately
	if delay <= 0 {
		r.timersMutex.Unlock()
		logger.Infow("🗑️  REAPING EXPIRED RESOURCE",
			zap.String("resource", resource.GetName()),
			zap.String("kind", resource.GetKind()),
			zap.String("namespace", resource.GetNamespace()),
			zap.Int64("ttlSeconds", ttlSeconds))

		err := r.dynamicClient.Resource(gvr).Namespace(resource.GetNamespace()).Delete(ctx, resource.GetName(), metav1.DeleteOptions{})
		if err != nil {
			logger.Errorw("❌ Failed to delete expired resource", zap.Error(err))
		} else {
			logger.Infow("✅ Successfully deleted expired resource",
				zap.String("resource", resource.GetName()))
		}
		return
	}

	// Schedule timer for exact expiration time
	timer := time.AfterFunc(delay, func() {
		logger.Infow("🗑️  REAPING EXPIRED RESOURCE (Timer)",
			zap.String("resource", resource.GetName()),
			zap.String("kind", resource.GetKind()),
			zap.String("namespace", resource.GetNamespace()),
			zap.Int64("ttlSeconds", ttlSeconds))

		err := r.dynamicClient.Resource(gvr).Namespace(resource.GetNamespace()).Delete(context.Background(), resource.GetName(), metav1.DeleteOptions{})
		if err != nil {
			logger.Errorw("❌ Failed to delete expired resource", zap.Error(err))
		} else {
			logger.Infow("✅ Successfully deleted expired resource",
				zap.String("resource", resource.GetName()))
		}

		// Clean up timer
		r.timersMutex.Lock()
		delete(r.timers, resourceKey)
		r.timersMutex.Unlock()
	})

	r.timers[resourceKey] = timer
	r.timersMutex.Unlock()

	logger.Infow("⏰ Scheduled TTL deletion",
		zap.String("resource", resource.GetName()),
		zap.Duration("delay", delay),
		zap.Time("expirationTime", expirationTime))
}

func (r *Reconciler) isResourceFinished(resource *unstructured.Unstructured) bool {
	// Check common completion status patterns

	// Pattern 1: status.phase == "Succeeded" or "Failed" (common in Jobs, etc.)
	if phase, found, err := unstructured.NestedString(resource.Object, "status", "phase"); found && err == nil {
		return phase == "Succeeded" || phase == "Failed" || phase == "Completed"
	}

	// Pattern 2: status.conditions with type="Succeeded" and status="True"
	conditions, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
	if found && err == nil {
		for _, conditionInterface := range conditions {
			if condition, ok := conditionInterface.(map[string]interface{}); ok {
				condType, typeFound := condition["type"].(string)
				condStatus, statusFound := condition["status"].(string)
				if typeFound && statusFound {
					if (condType == "Succeeded" || condType == "Completed") && condStatus == "True" {
						return true
					}
				}
			}
		}
	}

	// Pattern 3: status.completionTime exists (indicates completion)
	_, found, err = unstructured.NestedString(resource.Object, "status", "completionTime")
	if found && err == nil {
		return true
	}

	return false
}

// getResourceName converts a Kind to a resource name (pluralized, lowercase)
func getResourceName(kind string) string {
	// Simple pluralization - in a real implementation, you might want to use
	// a more sophisticated approach or discovery client
	lower := strings.ToLower(kind)
	if strings.HasSuffix(lower, "y") {
		return strings.TrimSuffix(lower, "y") + "ies"
	}
	if strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") {
		return lower + "es"
	}
	return lower + "s"
}

// Promote implements reconciler.LeaderAware
func (r *Reconciler) Promote(bkt reconciler.Bucket, enq func(reconciler.Bucket, types.NamespacedName)) error {
	// This is called when we become the leader.
	// For this simple controller, we don't need to do anything special.
	return nil
}

// Demote implements reconciler.LeaderAware
func (r *Reconciler) Demote(bkt reconciler.Bucket) {
	// This is called when we are no longer the leader.
	// For this simple controller, we don't need to do anything special.
}
