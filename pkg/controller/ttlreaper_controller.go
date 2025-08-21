/*
Copyright 2024 The TTL Reaper Authors.

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

package controller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ttlreaperv1alpha1 "github.com/infernus01/ttl-reaper/pkg/apis/ttlreaper/v1alpha1"
)

// TTLReaperReconciler reconciles a TTLReaperConfig object
type TTLReaperReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	DynamicClient dynamic.Interface
}

const (
	TTLReaperFinalizer = "ttlreaper.io/finalizer"
	DefaultTTLPath     = "spec.ttlSecondsAfterFinished"
	DefaultInterval    = 300 // 5 minutes
)

//+kubebuilder:rbac:groups=ttlreaper.io,resources=ttlreaperconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ttlreaper.io,resources=ttlreaperconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;delete

// Reconcile reconciles TTLReaperConfig resources
func (r *TTLReaperReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the TTLReaperConfig instance
	config := &ttlreaperv1alpha1.TTLReaperConfig{}
	err := r.Get(ctx, req.NamespacedName, config)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("TTLReaperConfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get TTLReaperConfig")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if config.ObjectMeta.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, config)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(config, TTLReaperFinalizer) {
		controllerutil.AddFinalizer(config, TTLReaperFinalizer)
		return ctrl.Result{}, r.Update(ctx, config)
	}

	// Process TTL cleanup
	result, err := r.processTTLCleanup(ctx, config)
	if err != nil {
		logger.Error(err, "Failed to process TTL cleanup")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	return result, nil
}

// processTTLCleanup handles the main TTL cleanup logic
func (r *TTLReaperReconciler) processTTLCleanup(ctx context.Context, config *ttlreaperv1alpha1.TTLReaperConfig) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Determine target namespace
	targetNamespace := config.Spec.TargetNamespace
	if targetNamespace == "" {
		targetNamespace = config.Namespace
	}

	// Create GroupVersionResource for the target kind
	gvr := schema.GroupVersionResource{
		Group:    getGroupFromAPIVersion(config.Spec.TargetAPIVersion),
		Version:  getVersionFromAPIVersion(config.Spec.TargetAPIVersion),
		Resource: getResourceFromKind(config.Spec.TargetKind),
	}

	// List target resources
	resourceList, err := r.DynamicClient.Resource(gvr).Namespace(targetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list target resources: %w", err)
	}

	ttlFieldPath := config.Spec.TTLFieldPath
	if ttlFieldPath == "" {
		ttlFieldPath = DefaultTTLPath
	}

	var processedCount, deletedCount int32
	now := time.Now()

	for _, resource := range resourceList.Items {
		processedCount++

		// Get TTL value from the resource
		ttlValue, found := getNestedField(&resource, ttlFieldPath)
		if !found {
			continue
		}

		ttlSeconds, err := convertToInt64(ttlValue)
		if err != nil {
			logger.Error(err, "Failed to convert TTL value to int64", "resource", resource.GetName(), "ttl", ttlValue)
			continue
		}

		// Check if resource should be deleted based on TTL
		if shouldDeleteResource(&resource, ttlSeconds, now) {
			logger.Info("Deleting expired resource", "kind", config.Spec.TargetKind, "name", resource.GetName(), "namespace", resource.GetNamespace())

			err := r.DynamicClient.Resource(gvr).Namespace(resource.GetNamespace()).Delete(ctx, resource.GetName(), metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete resource", "name", resource.GetName())
				continue
			}
			deletedCount++
		}
	}

	logger.Info("TTL cleanup completed", "processed", processedCount, "deleted", deletedCount)

	return r.scheduleNextReconcile(config), nil
}

// shouldDeleteResource determines if a resource should be deleted based on its TTL
func shouldDeleteResource(resource *unstructured.Unstructured, ttlSeconds int64, now time.Time) bool {
	// Check if resource has finished (based on status.conditions or completionTime)
	finished := isResourceFinished(resource)
	if !finished {
		return false
	}

	// Get completion time
	completionTime := getResourceCompletionTime(resource)
	if completionTime.IsZero() {
		return false
	}

	// Check if TTL has expired
	expirationTime := completionTime.Add(time.Duration(ttlSeconds) * time.Second)
	return now.After(expirationTime)
}

// isResourceFinished checks if a resource has finished execution
func isResourceFinished(resource *unstructured.Unstructured) bool {
	// Check status.conditions for completion
	conditions, found, _ := unstructured.NestedSlice(resource.Object, "status", "conditions")
	if found {
		for _, condition := range conditions {
			if condMap, ok := condition.(map[string]interface{}); ok {
				if condType, found := condMap["type"]; found && condType == "Succeeded" {
					if status, found := condMap["status"]; found && status == "True" {
						return true
					}
				}
			}
		}
	}

	// Check for completionTime field
	_, found, _ = unstructured.NestedString(resource.Object, "status", "completionTime")
	return found
}

// getResourceCompletionTime extracts the completion time from a resource
func getResourceCompletionTime(resource *unstructured.Unstructured) time.Time {
	// Try completionTime field first
	if completionTimeStr, found, _ := unstructured.NestedString(resource.Object, "status", "completionTime"); found {
		if completionTime, err := time.Parse(time.RFC3339, completionTimeStr); err == nil {
			return completionTime
		}
	}

	// Fallback to conditions lastTransitionTime
	conditions, found, _ := unstructured.NestedSlice(resource.Object, "status", "conditions")
	if found {
		for _, condition := range conditions {
			if condMap, ok := condition.(map[string]interface{}); ok {
				if condType, found := condMap["type"]; found && condType == "Succeeded" {
					if lastTransitionTime, found := condMap["lastTransitionTime"]; found {
						if timeStr, ok := lastTransitionTime.(string); ok {
							if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
								return parsedTime
							}
						}
					}
				}
			}
		}
	}

	return time.Time{}
}

// handleDeletion handles the deletion of TTLReaperConfig
func (r *TTLReaperReconciler) handleDeletion(ctx context.Context, config *ttlreaperv1alpha1.TTLReaperConfig) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(config, TTLReaperFinalizer) {
		controllerutil.RemoveFinalizer(config, TTLReaperFinalizer)
		return ctrl.Result{}, r.Update(ctx, config)
	}
	return ctrl.Result{}, nil
}

// scheduleNextReconcile determines when to schedule the next reconciliation
func (r *TTLReaperReconciler) scheduleNextReconcile(config *ttlreaperv1alpha1.TTLReaperConfig) ctrl.Result {
	interval := DefaultInterval
	if config.Spec.CheckInterval != nil {
		interval = int(*config.Spec.CheckInterval)
	}
	return ctrl.Result{RequeueAfter: time.Duration(interval) * time.Second}
}

// Helper functions
func getNestedField(obj *unstructured.Unstructured, fieldPath string) (interface{}, bool) {
	// Parse the field path (e.g., "spec.ttlSecondsAfterFinished" or "metadata.annotations.ttl-seconds")
	parts := split(fieldPath, ".")
	if len(parts) == 0 {
		return nil, false
	}

	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, parts...)
	if err != nil {
		return nil, false
	}
	return value, found
}

func convertToInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", value)
	}
}

func getGroupFromAPIVersion(apiVersion string) string {
	if apiVersion == "" {
		return ""
	}
	parts := split(apiVersion, "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

func getVersionFromAPIVersion(apiVersion string) string {
	if apiVersion == "" {
		return ""
	}
	parts := split(apiVersion, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}

func getResourceFromKind(kind string) string {
	// Simple pluralization - in production you'd want a more sophisticated approach
	if kind == "" {
		return ""
	}
	return kind + "s" // Basic pluralization
}

func split(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

// SetupWithManager sets up the controller with the Manager
func (r *TTLReaperReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ttlreaperv1alpha1.TTLReaperConfig{}).
		Complete(r)
}
