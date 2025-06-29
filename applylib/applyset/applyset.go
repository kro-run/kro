// Copyright 2025 The Kube Resource Orchestrator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Applylib is inspired from:
//  * kubectl pkg/cmd/apply/applyset.go
//  * kubebuilder-declarative-pattern/applylib
//  * Creating a simpler, self-contained version of the library that is purpose built for controllers.

package applyset

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"reflect"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

type ApplySetTooling struct {
	Name    string
	Version string
}

type ApplySet interface {
	Add(ctx context.Context, obj ApplyableObject) error
	Apply(ctx context.Context, prune bool) (*ApplyResult, error)
	DryRun(ctx context.Context, prune bool) (*ApplyResult, error)
}

type ApplySetConfig struct {
	ToolLabels   map[string]string
	FieldManager string
	ToolingID    ApplySetTooling

	// Callbacks
	LoadCallback        func(obj ApplyableObject, clusterValue *unstructured.Unstructured) error
	AfterApplyCallback  func(obj AppliedObject) error
	BeforePruneCallback func(obj PrunedObject) error
}

// NewApplySet creates a new ApplySet
// parent object is expected to be the current one existing in the cluster
func NewApplySet(
	parent *unstructured.Unstructured,
	restMapper meta.RESTMapper,
	dynamicClient dynamic.Interface,
	config ApplySetConfig,
) (ApplySet, error) {
	force := true
	if config.ToolingID == (ApplySetTooling{}) {
		return nil, fmt.Errorf("toolingID is required")
	}
	if config.FieldManager == "" {
		return nil, fmt.Errorf("fieldManager is required")
	}
	aset := &applySet{
		parent:              parent,
		toolingID:           config.ToolingID,
		toolLabels:          config.ToolLabels,
		desired:             NewTracker(),
		desiredRestMappings: make(map[schema.GroupKind]*meta.RESTMapping),
		desiredNamespaces:   sets.Set[string]{},
		clientSet: clientSet{
			restMapper:    restMapper,
			dynamicClient: dynamicClient,
		},
		serverOptions: serverOptions{
			applyOptions: metav1.ApplyOptions{
				FieldManager: config.FieldManager,
				Force:        force,
			},
			//deleteOptions: metav1.DeleteOptions{},
		},
		callbacks: callbacks{
			loadFn:        config.LoadCallback,
			afterApplyFn:  config.AfterApplyCallback,
			beforePruneFn: config.BeforePruneCallback,
		},
	}

	gvk := parent.GroupVersionKind()
	gk := gvk.GroupKind()
	restMapping, err := aset.restMapper.RESTMapping(gk, gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("error getting rest mapping for parent kind %v: %w", gvk, err)
	}
	if restMapping == nil {
		return nil, fmt.Errorf("rest mapping not found for parent kind %v", gvk)
	}
	if restMapping.Scope.Name() == meta.RESTScopeNameNamespace {
		aset.parentClient = aset.dynamicClient.Resource(restMapping.Resource).Namespace(parent.GetNamespace())
	} else {
		aset.parentClient = aset.dynamicClient.Resource(restMapping.Resource)
	}

	return aset, nil
}

type callbacks struct {
	loadFn        func(obj ApplyableObject, observed *unstructured.Unstructured) error
	afterApplyFn  func(obj AppliedObject) error
	beforePruneFn func(obj PrunedObject) error
}

type serverOptions struct {
	// patchOptions holds the options used when applying, in particular the fieldManager
	applyOptions metav1.ApplyOptions

	// deleteOptions holds the options used when pruning
	deleteOptions metav1.DeleteOptions
}

type clientSet struct {
	// client is the dynamic kubernetes client used to apply objects to the k8s cluster.
	dynamicClient dynamic.Interface

	// ParentClient is the controller runtime client used to apply parent.
	parentClient dynamic.ResourceInterface
	// restMapper is used to map object kind to resources, and to know if objects are cluster-scoped.
	restMapper meta.RESTMapper
}

// ApplySet tracks the information about an applyset apply/prune
type applySet struct {
	// Parent provides the necessary methods to determine a
	// ApplySet parent object, which can be used to find out all the on-track
	// deployment manifests.
	parent *unstructured.Unstructured

	// toolingID is the value to be used and validated in the applyset.kubernetes.io/tooling annotation.
	toolingID ApplySetTooling

	// toolLabels is a map of tool provided labels to be applied to the resources
	toolLabels map[string]string

	// current labels and annotations of the parent before the apply operation
	currentLabels      map[string]string
	currentAnnotations map[string]string

	// desiredRestMappings is a map of object key to object.RESTMapping
	desiredRestMappings map[schema.GroupKind]*meta.RESTMapping
	desiredNamespaces   sets.Set[string]

	desired *tracker
	clientSet
	serverOptions
	callbacks
}

func (t ApplySetTooling) String() string {
	return fmt.Sprintf("%s/%s", t.Name, t.Version)
}

func (a *applySet) validateAndCacheNamespace(obj ApplyableObject, restMapping *meta.RESTMapping) error {
	// Ensure object namespace is correct for the scope
	gvk := obj.GroupVersionKind()
	switch restMapping.Scope.Name() {
	case meta.RESTScopeNameNamespace:
		// TODO (barney-s): Should we allow empty namespace for namespace-scoped objects?
		//if obj.GetNamespace() == "" {
		//	return fmt.Errorf("namespace was not provided for namespace-scoped object %v %v", gvk, obj.GetName())
		//}
		a.desiredNamespaces.Insert(obj.GetNamespace())
	case meta.RESTScopeNameRoot:
		if obj.GetNamespace() != "" {
			return fmt.Errorf("namespace was provided for cluster-scoped object %v %v", gvk, obj.GetName())
		}

	default:
		// Internal error ... this is panic-level
		return fmt.Errorf("unknown scope for gvk %s: %q", gvk, restMapping.Scope.Name())
	}
	return nil
}

func (a *applySet) validateAndCacheRestMapping(obj ApplyableObject) (*meta.RESTMapping, error) {
	gvk := obj.GroupVersionKind()
	gk := gvk.GroupKind()
	// Ensure a rest mapping exists for the object
	_, found := a.desiredRestMappings[gk]
	if !found {
		restMapping, err := a.restMapper.RESTMapping(gk, gvk.Version)
		if err != nil {
			return nil, fmt.Errorf("error getting rest mapping for %v: %w", gvk, err)
		}
		if restMapping == nil {
			return nil, fmt.Errorf("rest mapping not found for %v", gvk)
		}
		a.desiredRestMappings[gk] = restMapping
	}

	return a.desiredRestMappings[gk], nil
}

func (a *applySet) ResourceClient(obj Applyable) dynamic.ResourceInterface {
	restMapping, ok := a.desiredRestMappings[obj.GroupVersionKind().GroupKind()]
	if !ok {
		return nil
	}
	dynResource := a.dynamicClient.Resource(restMapping.Resource)
	if restMapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = a.parent.GetNamespace()
		}
		if ns == "" {
			ns = metav1.NamespaceDefault
		}
		return dynResource.Namespace(ns)
	}
	return dynResource
}

func (a *applySet) Add(ctx context.Context, obj ApplyableObject) error {
	restMapping, err := a.validateAndCacheRestMapping(obj)
	if err != nil {
		return err
	}
	if err := a.validateAndCacheNamespace(obj, restMapping); err != nil {
		return err
	}
	obj.SetLabels(a.InjectApplysetLabels(a.InjectToolLabels(obj.GetLabels())))

	dynResource := a.ResourceClient(obj)
	// This should never happen, but if it does, we want to know about it.
	if dynResource == nil {
		return fmt.Errorf("FATAL: rest mapping not found for %v", obj.GroupVersionKind())
	}
	observed, err := dynResource.Get(ctx,
		obj.GetName(),
		metav1.GetOptions{},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			observed = nil
		} else {
			return fmt.Errorf("error getting object from cluster: %w", err)
		}
	}
	if observed != nil {
		// Record the last read revision of the object.
		obj.lastReadRevision = observed.GetResourceVersion()
	}

	if err := a.desired.Add(obj); err != nil {
		return err
	}
	// If a load callback is provided, we will load the object from the cluster.
	if a.loadFn != nil {
		if err := a.loadFn(obj, observed); err != nil {
			return err
		}
	}
	return nil
}

// ID is the label value that we are using to identify this applyset.
// Format: base64(sha256(<name>.<namespace>.<kind>.<apiVersion>)), using the URL safe encoding of RFC4648.

func (a *applySet) ID() string {
	unencoded := strings.Join([]string{
		a.parent.GetName(),
		a.parent.GetNamespace(),
		a.parent.GetKind(),
		a.parent.GroupVersionKind().Group,
	}, ApplySetIDPartDelimiter)
	hashed := sha256.Sum256([]byte(unencoded))
	b64 := base64.RawURLEncoding.EncodeToString(hashed[:])
	// Label values must start and end with alphanumeric values, so add a known-safe prefix and suffix.
	return fmt.Sprintf(V1ApplySetIdFormat, b64)
}

func (a *applySet) InjectToolLabels(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	if a.toolLabels != nil {
		for k, v := range a.toolLabels {
			labels[k] = v
		}
	}
	return labels
}

func (a *applySet) InjectApplysetLabels(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[ApplysetPartOfLabel] = a.ID()
	return labels
}

type ApplySetUpdateMode string

var updateToLatestSet ApplySetUpdateMode = "latest"
var updateToSuperset ApplySetUpdateMode = "superset"

// updateParentLabelsAndAnnotations updates the parent labels and annotations.
func (a *applySet) updateParentLabelsAndAnnotations(ctx context.Context, mode ApplySetUpdateMode) error {
	original, err := meta.Accessor(a.parent.DeepCopyObject())
	if err != nil {
		return err
	}

	parentPatch := &unstructured.Unstructured{}
	parentPatch.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": a.parent.GetAPIVersion(),
		"kind":       a.parent.GetKind(),
		"metadata": map[string]interface{}{
			"name":      a.parent.GetName(),
			"namespace": a.parent.GetNamespace(),
		},
	})
	// Generate and append the desired labels to the parent labels
	desiredLabels := a.desiredParentLabels()
	labels := a.parent.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range desiredLabels {
		labels[k] = v
	}
	parentPatch.SetLabels(labels)

	// Get the desired annotations and append them to the parent
	desiredAnnotations := a.desiredParentAnnotations(mode == updateToSuperset)
	annotations := a.parent.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	for k, v := range desiredAnnotations {
		annotations[k] = v
	}
	parentPatch.SetAnnotations(annotations)

	options := metav1.ApplyOptions{
		FieldManager: "kro-parent-labeller",
		Force:        false,
	}

	// update parent in the cluster.
	if !reflect.DeepEqual(original.GetLabels(), parentPatch.GetLabels()) ||
		!reflect.DeepEqual(original.GetAnnotations(), parentPatch.GetAnnotations()) {
		if _, err := a.parentClient.Apply(ctx, a.parent.GetName(), parentPatch, options); err != nil {
			return fmt.Errorf("error updating parent %w", err)
		}
	}
	return nil
}

func (a *applySet) desiredParentLabels() map[string]string {
	labels := make(map[string]string)
	labels[ApplySetParentIDLabel] = a.ID()
	return labels
}

func (a *applySet) desiredParentAnnotations(includeCurrent bool) map[string]string {
	annotations := make(map[string]string)
	annotations[ApplySetToolingAnnotation] = a.toolingID.String()

	// Generate sorted comma-separated list of GKs
	gks := sets.Set[string]{}
	for gk := range a.desiredRestMappings {
		gks.Insert(gk.String())
	}
	if includeCurrent {
		for _, gk := range strings.Split(a.currentAnnotations[ApplySetGKsAnnotation], ",") {
			gks.Insert(gk)
		}
	}
	gksList := gks.UnsortedList()
	sort.Strings(gksList)
	annotations[ApplySetGKsAnnotation] = strings.Join(gksList, ",")

	// Generate sorted comma-separated list of namespaces
	nss := a.desiredNamespaces.Clone()
	if includeCurrent {
		for _, ns := range strings.Split(a.currentAnnotations[ApplySetAdditionalNamespacesAnnotation], ",") {
			nss.Insert(ns)
		}
	}
	nsList := a.desiredNamespaces.UnsortedList()
	sort.Strings(nsList)
	annotations[ApplySetAdditionalNamespacesAnnotation] = strings.Join(nsList, ",")
	return annotations
}

func (a *applySet) apply(ctx context.Context, dryRun bool) (*ApplyResult, error) {
	results := &ApplyResult{Desired: a.desired.Len()}

	// If dryRun is true, we will not update the parent labels and annotations.
	if !dryRun {
		parent, err := meta.Accessor(a.parent)
		if err != nil {
			return results, fmt.Errorf("unable to get parent: %w", err)
		}
		// Record the current labels and annotations
		a.currentLabels = parent.GetLabels()
		a.currentAnnotations = parent.GetAnnotations()

		// We will ensure the parent is updated with the latest applyset before applying the resources.
		if err := a.updateParentLabelsAndAnnotations(ctx, updateToSuperset); err != nil {
			return results, fmt.Errorf("unable to update Parent: %w", err)
		}
	}

	options := a.applyOptions
	if dryRun {
		options.DryRun = []string{"All"}
	}
	for i := range a.desired.objects {
		obj := a.desired.objects[i]

		dynResource := a.ResourceClient(obj)
		// This should never happen, but if it does, we want to know about it.
		if dynResource == nil {
			return results, fmt.Errorf("FATAL: rest mapping not found for %v", obj.GroupVersionKind())
		}
		unstructuredObj, ok := obj.Applyable.(*unstructured.Unstructured)
		if !ok {
			return results, fmt.Errorf("FATAL: object %v is not an unstructured.Unstructured", obj.GroupVersionKind())
		}
		lastApplied, err := dynResource.Apply(ctx, obj.GetName(), unstructuredObj, options)
		ao := results.recordApplied(a.desired.objects[i], lastApplied, err)

		if a.afterApplyFn != nil {
			if err := a.afterApplyFn(ao); err != nil {
				return results, fmt.Errorf("error after apply: %w", err)
			}
		}

		// TODO: Implement health computation after apply
		//message := ""
		//tracker.isHealthy, message, err = a.computeHealth(lastApplied)
		//results.reportHealth(gvk, nn, lastApplied, tracker.isHealthy, message, err)
	}

	return results, nil
}

func (a *applySet) prune(ctx context.Context, results *ApplyResult, dryRun bool) (*ApplyResult, error) {
	pruneObjects, err := a.FindAllObjectsToPrune(ctx, a.dynamicClient, results.AppliedUIDs())
	if err != nil {
		return results, err
	}
	options := a.deleteOptions
	if dryRun {
		options.DryRun = []string{"All"}
	}
	for i := range pruneObjects {
		pruneObject := &pruneObjects[i]
		name := pruneObject.Name
		namespace := pruneObject.Namespace
		mapping := pruneObject.Mapping

		// TODO: Why is it always using namesapce ?
		po := PrunedObject{
			PruneObject: pruneObjects[i],
		}
		if a.beforePruneFn != nil {
			if err := a.beforePruneFn(po); err != nil {
				return results, fmt.Errorf("error from before-prune callback: %w", err)
			}
		}
		err := a.dynamicClient.Resource(mapping.Resource).Namespace(namespace).Delete(ctx, name, options)
		po.Error = err
		results.PrunedObjects = append(results.PrunedObjects, po)
	}

	if !dryRun {
		// "latest" mode updates the parent "applyset.kubernetes.io/contains-group-resources" annotations
		// to only contain the current manifest GVRs.
		if err := a.updateParentLabelsAndAnnotations(ctx, updateToLatestSet); err != nil {
			return results, fmt.Errorf("unable to update Parent: %w", err)
		}
	}

	return results, nil
}

func (a *applySet) applyAndPrune(ctx context.Context, prune bool, dryRun bool) (*ApplyResult, error) {
	results, err := a.apply(ctx, dryRun)
	if err != nil {
		return results, err
	}

	if !prune {
		return results, nil
	}

	return a.prune(ctx, results, dryRun)
}

func (a *applySet) Apply(ctx context.Context, prune bool) (*ApplyResult, error) {
	return a.applyAndPrune(ctx, prune, false)
}

func (a *applySet) DryRun(ctx context.Context, prune bool) (*ApplyResult, error) {
	return a.applyAndPrune(ctx, prune, true)
}
