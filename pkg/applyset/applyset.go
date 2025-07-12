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
//  * KEP describing applyset:
//     https://git.k8s.io/enhancements/keps/sig-cli/3659-kubectl-apply-prune#design-details-applyset-specification

package applyset

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

type ToolingID struct {
	Name    string
	Version string
}

func (t ToolingID) String() string {
	return fmt.Sprintf("%s/%s", t.Name, t.Version)
}

type Set interface {
	Add(ctx context.Context, obj ApplyableObject) error
	Apply(ctx context.Context, prune bool) (*ApplyResult, error)
	DryRun(ctx context.Context, prune bool) (*ApplyResult, error)
}

type Config struct {
	ToolLabels   map[string]string
	FieldManager string
	ToolingID    ToolingID
	Log          logr.Logger

	// Callbacks
	LoadCallback        func(obj ApplyableObject, clusterValue *unstructured.Unstructured) error
	AfterApplyCallback  func(obj AppliedObject) error
	BeforePruneCallback func(obj PrunedObject) error
}

// New creates a new ApplySet
// parent object is expected to be the current one existing in the cluster
func New(
	parent *unstructured.Unstructured,
	restMapper meta.RESTMapper,
	dynamicClient dynamic.Interface,
	config Config,
) (Set, error) {
	if config.ToolingID == (ToolingID{}) {
		return nil, fmt.Errorf("toolingID is required")
	}
	if config.FieldManager == "" {
		return nil, fmt.Errorf("fieldManager is required")
	}
	aset := &applySet{
		parent:              parent,
		toolingID:           config.ToolingID,
		toolLabels:          config.ToolLabels,
		fieldManager:        config.FieldManager,
		desired:             NewTracker(),
		desiredRESTMappings: make(map[schema.GroupKind]*meta.RESTMapping),
		desiredNamespaces:   sets.Set[string]{},
		supersetNamespaces:  sets.Set[string]{},
		supersetGKs:         sets.Set[string]{},
		clientSet: clientSet{
			restMapper:    restMapper,
			dynamicClient: dynamicClient,
		},
		serverOptions: serverOptions{
			applyOptions: metav1.ApplyOptions{
				FieldManager: config.FieldManager,
				Force:        true,
			},
			//deleteOptions: metav1.DeleteOptions{},
		},
		callbacks: callbacks{
			loadFn:        config.LoadCallback,
			afterApplyFn:  config.AfterApplyCallback,
			beforePruneFn: config.BeforePruneCallback,
		},
		log: config.Log,
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
	// applyOptions holds the options used when applying, in particular the fieldManager
	applyOptions metav1.ApplyOptions

	// deleteOptions holds the options used when pruning.
	deleteOptions metav1.DeleteOptions
}

type clientSet struct {
	// dynamicClient is the dynamic kubernetes client used to apply objects to the k8s cluster.
	dynamicClient dynamic.Interface

	// parentClient is the controller runtime client used to apply parent.
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
	toolingID ToolingID

	// fieldManager is the name of the field manager that will be used to apply the resources.
	fieldManager string

	// toolLabels is a map of tool provided labels to be applied to the resources
	toolLabels map[string]string

	// current labels and annotations of the parent before the apply operation
	currentLabels      map[string]string
	currentAnnotations map[string]string

	// set of applyset object rest mappings
	desiredRESTMappings map[schema.GroupKind]*meta.RESTMapping
	// set of applyset object namespaces
	desiredNamespaces sets.Set[string]

	// superset of desired and old namespaces
	supersetNamespaces sets.Set[string]
	// superset of desired and old GKs
	supersetGKs sets.Set[string]

	desired *tracker
	clientSet
	serverOptions
	callbacks

	log logr.Logger
}

func (a *applySet) validateAndCacheNamespace(obj ApplyableObject, restMapping *meta.RESTMapping) error {
	// Ensure object namespace is correct for the scope
	gvk := obj.GroupVersionKind()
	switch restMapping.Scope.Name() {
	case meta.RESTScopeNameNamespace:
		// If empty use the parent's namespace for the object.
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = a.parent.GetNamespace()
		}
		a.desiredNamespaces.Insert(namespace)
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
	_, found := a.desiredRESTMappings[gk]
	if !found {
		restMapping, err := a.restMapper.RESTMapping(gk, gvk.Version)
		if err != nil {
			return nil, fmt.Errorf("error getting rest mapping for %v: %w", gvk, err)
		}
		if restMapping == nil {
			return nil, fmt.Errorf("rest mapping not found for %v", gvk)
		}
		a.desiredRESTMappings[gk] = restMapping
	}

	return a.desiredRESTMappings[gk], nil
}

func (a *applySet) resourceClient(obj Applyable) dynamic.ResourceInterface {
	restMapping, ok := a.desiredRESTMappings[obj.GroupVersionKind().GroupKind()]
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
	obj.SetLabels(a.InjectApplysetLabels(a.injectToolLabels(obj.GetLabels())))

	dynResource := a.resourceClient(obj)
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
	a.log.V(2).Info("adding object to applyset", "object", obj.String(), "cluster-revision", obj.lastReadRevision)

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

func (a *applySet) injectToolLabels(labels map[string]string) map[string]string {
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

type applySetUpdateMode string

var updateToLatestSet applySetUpdateMode = "latest"
var updateToSuperset applySetUpdateMode = "superset"

// updateParentLabelsAndAnnotations updates the parent labels and annotations.
func (a *applySet) updateParentLabelsAndAnnotations(ctx context.Context, mode applySetUpdateMode) (sets.Set[string],
	sets.Set[string], error) {
	original, err := meta.Accessor(a.parent.DeepCopyObject())
	if err != nil {
		return nil, nil, err
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
	desiredAnnotations, returnNamespaces, returnGKs := a.desiredParentAnnotations(mode == updateToSuperset)
	annotations := a.parent.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	for k, v := range desiredAnnotations {
		annotations[k] = v
	}
	parentPatch.SetAnnotations(annotations)

	options := metav1.ApplyOptions{
		FieldManager: a.fieldManager + "parent-labeller",
		Force:        false,
	}

	// update parent in the cluster.
	if !reflect.DeepEqual(original.GetLabels(), parentPatch.GetLabels()) ||
		!reflect.DeepEqual(original.GetAnnotations(), parentPatch.GetAnnotations()) {
		if _, err := a.parentClient.Apply(ctx, a.parent.GetName(), parentPatch, options); err != nil {
			return nil, nil, fmt.Errorf("error updating parent %w", err)
		}
		a.log.V(2).Info("updated parent labels and annotations", "parent-name", a.parent.GetName(),
			"parent-namespace", a.parent.GetNamespace(),
			"parent-gvk", a.parent.GroupVersionKind(),
			"parent-labels", desiredLabels, "parent-annotations", desiredAnnotations)
	}
	return returnNamespaces, returnGKs, nil
}

func (a *applySet) desiredParentLabels() map[string]string {
	labels := make(map[string]string)
	labels[ApplySetParentIDLabel] = a.ID()
	return labels
}

// Return the annotations as well as the set of namespaces and GKs
func (a *applySet) desiredParentAnnotations(includeCurrent bool) (map[string]string,
	sets.Set[string], sets.Set[string]) {
	annotations := make(map[string]string)
	annotations[ApplySetToolingAnnotation] = a.toolingID.String()

	// Generate sorted comma-separated list of GKs
	gks := sets.Set[string]{}
	for gk := range a.desiredRESTMappings {
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
	return annotations, nss, gks
}

func (a *applySet) apply(ctx context.Context, dryRun bool) (*ApplyResult, error) {
	results := &ApplyResult{DesiredCount: a.desired.Len()}

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
		a.supersetNamespaces, a.supersetGKs, err = a.updateParentLabelsAndAnnotations(ctx, updateToSuperset)
		if err != nil {
			return results, fmt.Errorf("unable to update Parent: %w", err)
		}
	}

	options := a.applyOptions
	if dryRun {
		options.DryRun = []string{"All"}
	}
	for _, obj := range a.desired.objects {

		dynResource := a.resourceClient(obj)
		// This should never happen, but if it does, we want to know about it.
		if dynResource == nil {
			return results, fmt.Errorf("FATAL: rest mapping not found for %v", obj.GroupVersionKind())
		}
		lastApplied, err := dynResource.Apply(ctx, obj.GetName(), obj.Unstructured, options)
		ao := results.recordApplied(obj, lastApplied, err)
		a.log.V(2).Info("applied object", "object", obj.String(), "applied-revision", lastApplied.GetResourceVersion(),
			"error", err)

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
		name := pruneObjects[i].GetName()
		namespace := pruneObjects[i].GetNamespace()
		mapping := pruneObjects[i].Mapping

		// TODO: Why is it always using namesapce ?
		if a.beforePruneFn != nil {
			if err := a.beforePruneFn(PrunedObject{
				PruneObject: pruneObjects[i],
			}); err != nil {
				return results, fmt.Errorf("error from before-prune callback: %w", err)
			}
		}
		err := a.dynamicClient.Resource(mapping.Resource).Namespace(namespace).Delete(ctx, name, options)
		results.recordPruned(pruneObjects[i], err)
		a.log.V(2).Info("pruned object", "object", pruneObjects[i].String(), "error", err)
	}

	if !dryRun {
		// "latest" mode updates the parent "applyset.kubernetes.io/contains-group-resources" annotations
		// to only contain the current manifest GVRs.
		if _, _, err := a.updateParentLabelsAndAnnotations(ctx, updateToLatestSet); err != nil {
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
