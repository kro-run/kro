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

package applyset

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplySetTooling struct {
	Name    string
	Version string
}

// Parent is aimed for adaption. We want users to us Parent rather than the third-party/forked ParentRef directly.
// This gives us more flexibility to change the third-party/forked without introducing breaking changes to users.
type Parent interface {
	GroupVersionKind() schema.GroupVersionKind
	Name() string
	Namespace() string
	IsNamespaced() bool
	RESTMapping() *meta.RESTMapping
	GetSubject() runtime.Object
	GetSubjectKey() client.ObjectKey
}

// ApplyableObject is implemented by objects that can be applied to the cluster.
// We don't need much, so this might allow for more efficient implementations in future.
type Applyable interface {
	// GroupVersionKind returns the GroupVersionKind structure describing the type of the object
	GroupVersionKind() schema.GroupVersionKind
	// GetNamespace returns the namespace of the object
	GetNamespace() string
	// GetName returns the name of the object
	GetName() string

	// GetLabels returns the labels of the object
	GetLabels() map[string]string
	// SetLabels sets the labels of the object
	SetLabels(labels map[string]string)

	// The object should implement json marshalling
	json.Marshaler
}

type ApplySet interface {
	Add(ApplyableObject) error
	Apply(ctx context.Context) (*ApplyResult, error)
	ApplyAndPrune(ctx context.Context) (*ApplyResult, error)
}

// NewApplySet creates a new ApplySet
// parent object is expected to be the current one existing in the cluster
func NewApplySet(parent Parent, toolingID ApplySetTooling) ApplySet {
	return &applySet{
		parent:              parent,
		toolingID:           toolingID,
		desired:             NewTracker(),
		desiredRestMappings: make(map[schema.GroupKind]*meta.RESTMapping),
	}
}

type ServerOptions struct {
	// patchOptions holds the options used when applying, in particular the fieldManager
	patchOptions metav1.PatchOptions

	// deleteOptions holds the options used when pruning
	deleteOptions metav1.DeleteOptions
}

type ClientSet struct {
	// client is the dynamic kubernetes client used to apply objects to the k8s cluster.
	dynamicClient dynamic.Interface

	// ParentClient is the controller runtime client used to apply parent.
	parentClient client.Client
	// restMapper is used to map object kind to resources, and to know if objects are cluster-scoped.
	restMapper meta.RESTMapper
}

// ApplySet tracks the information about an applyset apply/prune
type applySet struct {
	// Parent provides the necessary methods to determine a
	// ApplySet parent object, which can be used to find out all the on-track
	// deployment manifests.
	parent Parent

	// toolingID is the value to be used and validated in the applyset.kubernetes.io/tooling annotation.
	toolingID ApplySetTooling

	// current labels and annotations of the parent before the apply operation
	currentLabels      map[string]string
	currentAnnotations map[string]string

	// desiredRestMappings is a map of object key to object.RESTMapping
	desiredRestMappings map[schema.GroupKind]*meta.RESTMapping
	desiredNamespaces   sets.Set[string]

	desired *tracker
	ClientSet
	ServerOptions
}

func (t ApplySetTooling) String() string {
	return fmt.Sprintf("%s/%s", t.Name, t.Version)
}

func (a *applySet) validateAndCacheNamespace(obj ApplyableObject, restMapping *meta.RESTMapping) error {
	// Ensure object namespace is correct for the scope
	gvk := obj.GroupVersionKind()
	switch restMapping.Scope.Name() {
	case meta.RESTScopeNameNamespace:
		if obj.GetNamespace() == "" {
			return fmt.Errorf("namespace was not provided for namespace-scoped object %v %v", gvk, obj.GetName())
		}
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
	restMapping, found := a.desiredRestMappings[gk]
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

	return restMapping, nil
}

func (a *applySet) DynamicResource(obj ApplyableObject, dynInterface dynamic.Interface) dynamic.ResourceInterface {
	restMapping, ok := a.desiredRestMappings[obj.GroupVersionKind().GroupKind()]
	if !ok {
		return nil
	}
	dynResource := dynInterface.Resource(restMapping.Resource)
	if restMapping.Scope.Name() == meta.RESTScopeNameNamespace {
		return dynResource.Namespace(obj.GetNamespace())
	}
	return dynResource
}

func (a *applySet) Add(obj ApplyableObject) error {
	restMapping, err := a.validateAndCacheRestMapping(obj)
	if err != nil {
		return err
	}
	if err := a.validateAndCacheNamespace(obj, restMapping); err != nil {
		return err
	}
	return a.desired.Add(obj)
}

// ID is the label value that we are using to identify this applyset.
// Format: base64(sha256(<name>.<namespace>.<kind>.<group>)), using the URL safe encoding of RFC4648.

func (a *applySet) ID() string {
	unencoded := strings.Join([]string{
		a.parent.Name(),
		a.parent.Namespace(),
		a.parent.GroupVersionKind().Kind,
		a.parent.GroupVersionKind().Group,
	}, ApplySetIDPartDelimiter)
	hashed := sha256.Sum256([]byte(unencoded))
	b64 := base64.RawURLEncoding.EncodeToString(hashed[:])
	// Label values must start and end with alphanumeric values, so add a known-safe prefix and suffix.
	return fmt.Sprintf(V1ApplySetIdFormat, b64)
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
	parent, err := meta.Accessor(a.parent.GetSubject())
	if err != nil {
		return err
	}

	original, err := meta.Accessor(a.parent.GetSubject().DeepCopyObject())
	if err != nil {
		return err
	}

	// Generate and append the desired labels to the parent labels
	desiredLabels := a.desiredLabels()
	labels := parent.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range desiredLabels {
		labels[k] = v
	}
	parent.SetLabels(labels)

	// Get the desired annotations and append them to the parent
	desiredAnnotations := a.desiredAnnotations(mode == updateToSuperset)
	annotations := parent.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	for k, v := range desiredAnnotations {
		annotations[k] = v
	}
	parent.SetAnnotations(annotations)

	// update parent in the cluster.
	if !reflect.DeepEqual(original.GetLabels(), parent.GetLabels()) ||
		!reflect.DeepEqual(original.GetAnnotations(), parent.GetAnnotations()) {
		if err := a.parentClient.Update(ctx, parent.(client.Object)); err != nil {
			return fmt.Errorf("error updating parent %w", err)
		}
	}
	return nil
}

func (a *applySet) desiredLabels() map[string]string {
	labels := make(map[string]string)
	labels[ApplysetPartOfLabel] = a.ID()
	return labels
}

func (a *applySet) desiredAnnotations(includeCurrent bool) map[string]string {
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

func (a *applySet) Apply(ctx context.Context) (*ApplyResult, error) {
	results := &ApplyResult{Desired: a.desired.Len()}

	parent, err := meta.Accessor(a.parent.GetSubject())
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

	for i := range a.desired.objects {
		obj := a.desired.objects[i]

		obj.SetLabels(a.InjectApplysetLabels(obj.GetLabels()))

		dynResource := a.DynamicResource(obj, a.dynamicClient)
		// This should never happen, but if it does, we want to know about it.
		if dynResource == nil {
			return results, fmt.Errorf("FATAL: dynamic resource not found for %v", obj.GroupVersionKind())
		}
		lastApplied, err := dynResource.Patch(ctx,
			obj.GetName(),
			types.ApplyPatchType, obj.Json(),
			a.patchOptions,
		)
		results.recordApplied(a.desired.objects[i], lastApplied, err)

		// TODO: Implement health computation after apply
		//message := ""
		//tracker.isHealthy, message, err = a.computeHealth(lastApplied)
		//results.reportHealth(gvk, nn, lastApplied, tracker.isHealthy, message, err)
	}

	if !results.Success() {
		return results, fmt.Errorf("Not all resources applied successfully")
	}
	return results, nil
}

func (a *applySet) prune(ctx context.Context, results *ApplyResult) (*ApplyResult, error) {
	pruneObjects, err := a.FindAllObjectsToPrune(ctx, a.dynamicClient, results.AppliedUIDs())
	if err != nil {
		return results, err
	}
	for i := range pruneObjects {
		pruneObject := &pruneObjects[i]
		name := pruneObject.Name
		namespace := pruneObject.Namespace
		mapping := pruneObject.Mapping

		// TODO: Why is it always using namesapce ?
		err := a.dynamicClient.Resource(mapping.Resource).Namespace(namespace).Delete(ctx, name, a.deleteOptions)
		results.recordPrune(pruneObjects[i], err)
	}
	// "latest" mode updates the parent "applyset.kubernetes.io/contains-group-resources" annotations
	// to only contain the current manifest GVRs.
	if err := a.updateParentLabelsAndAnnotations(ctx, updateToLatestSet); err != nil {
		return results, fmt.Errorf("unable to update Parent: %w", err)
	}
	return results, nil
}

func (a *applySet) ApplyAndPrune(ctx context.Context) (*ApplyResult, error) {
	results, err := a.Apply(ctx)
	if err != nil {
		return results, err
	}

	return a.prune(ctx, results)
}
