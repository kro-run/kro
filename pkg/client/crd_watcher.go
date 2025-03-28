// Copyright 2025 The Kube Resource Orchestrator Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/kro-run/kro/pkg/metadata"
)

// CRDWatcher watches for changes to CRDs managed by kro and triggers reconciliation
// when changes are detected.
type CRDWatcher struct {
	client    apiextensionsv1.CustomResourceDefinitionInterface
	log       logr.Logger
	informer  cache.SharedIndexInformer
	queue     workqueue.RateLimitingInterface
	callbacks map[string]CRDCallback
	mu        sync.RWMutex
	stopCh    chan struct{}
}

// CRDCallback is a function that is called when a CRD is modified.
type CRDCallback func(crd *v1.CustomResourceDefinition)

// CRDWatcherConfig contains configuration for the CRD watcher.
type CRDWatcherConfig struct {
	Client *apiextensionsv1.ApiextensionsV1Client
	Log    logr.Logger
}

// NewCRDWatcher creates a new CRD watcher.
func NewCRDWatcher(cfg CRDWatcherConfig) *CRDWatcher {
	return &CRDWatcher{
		client:    cfg.Client.CustomResourceDefinitions(),
		log:       cfg.Log.WithName("crd-watcher"),
		callbacks: make(map[string]CRDCallback),
		queue:     workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		stopCh:    make(chan struct{}),
	}
}

// Start starts the CRD watcher.
func (w *CRDWatcher) Start(ctx context.Context) error {
	w.log.Info("Starting CRD watcher")

	// Create a label selector to watch only CRDs managed by kro
	labelSelector := fmt.Sprintf("%s=true", metadata.OwnedLabel)

	// Create the informer
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
			options.LabelSelector = labelSelector
			return w.client.List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = labelSelector
			return w.client.Watch(ctx, options)
		},
	}

	w.informer = cache.NewSharedIndexInformer(
		lw,
		&v1.CustomResourceDefinition{},
		0, // No resync (we want to react only to actual changes)
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	// Set up event handlers
	w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: w.handleUpdate,
	})

	// Start the informer
	go w.informer.Run(w.stopCh)

	// Wait for the informer to sync
	if !cache.WaitForCacheSync(w.stopCh, w.informer.HasSynced) {
		return fmt.Errorf("failed to sync CRD informer cache")
	}

	// Start the worker
	go w.runWorker(ctx)

	w.log.Info("CRD watcher started successfully")
	return nil
}

// Stop stops the CRD watcher.
func (w *CRDWatcher) Stop() {
	w.log.Info("Stopping CRD watcher")
	close(w.stopCh)
	w.queue.ShutDown()
}

// RegisterCallback registers a callback for a specific CRD.
func (w *CRDWatcher) RegisterCallback(crdName string, callback CRDCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.log.V(1).Info("Registering callback for CRD", "crdName", crdName)
	w.callbacks[crdName] = callback
}

// UnregisterCallback unregisters a callback for a specific CRD.
func (w *CRDWatcher) UnregisterCallback(crdName string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.log.V(1).Info("Unregistering callback for CRD", "crdName", crdName)
	delete(w.callbacks, crdName)
}

// handleUpdate handles update events from the informer.
func (w *CRDWatcher) handleUpdate(oldObj, newObj interface{}) {
	oldCRD, ok := oldObj.(*v1.CustomResourceDefinition)
	if !ok {
		w.log.Error(nil, "Failed to cast old object to CRD")
		return
	}

	newCRD, ok := newObj.(*v1.CustomResourceDefinition)
	if !ok {
		w.log.Error(nil, "Failed to cast new object to CRD")
		return
	}

	// Skip if the resource version hasn't changed
	if oldCRD.ResourceVersion == newCRD.ResourceVersion {
		return
	}

	w.log.V(1).Info("CRD updated", "name", newCRD.Name, "oldResourceVersion", oldCRD.ResourceVersion, "newResourceVersion", newCRD.ResourceVersion)
	w.queue.Add(newCRD.Name)
}

// runWorker processes items from the queue.
func (w *CRDWatcher) runWorker(ctx context.Context) {
	for w.processNextItem(ctx) {
	}
}

// processNextItem processes a single item from the queue.
func (w *CRDWatcher) processNextItem(ctx context.Context) bool {
	// Get the next item from the queue
	key, quit := w.queue.Get()
	if quit {
		return false
	}
	defer w.queue.Done(key)

	// Process the item
	err := w.processCRD(ctx, key.(string))
	if err != nil {
		w.log.Error(err, "Error processing CRD", "name", key)
		w.queue.AddRateLimited(key)
		return true
	}

	// If we got here, the item was processed successfully
	w.queue.Forget(key)
	return true
}

// processCRD processes a single CRD.
func (w *CRDWatcher) processCRD(ctx context.Context, crdName string) error {
	// Get the CRD from the informer cache
	obj, exists, err := w.informer.GetIndexer().GetByKey(crdName)
	if err != nil {
		return fmt.Errorf("error getting CRD from cache: %w", err)
	}

	if !exists {
		w.log.V(1).Info("CRD no longer exists", "name", crdName)
		return nil
	}

	crd, ok := obj.(*v1.CustomResourceDefinition)
	if !ok {
		return fmt.Errorf("error casting object to CRD")
	}

	// Call the callback if registered
	w.mu.RLock()
	callback, exists := w.callbacks[crdName]
	w.mu.RUnlock()

	if exists {
		w.log.V(1).Info("Calling callback for CRD", "name", crdName)
		callback(crd)
	}

	return nil
}

// GetGVRFromCRD extracts the GroupVersionResource from a CRD.
func GetGVRFromCRD(crd *v1.CustomResourceDefinition) schema.GroupVersionResource {
	// Get the first version from the CRD
	version := crd.Spec.Versions[0].Name
	return schema.GroupVersionResource{
		Group:    crd.Spec.Group,
		Version:  version,
		Resource: crd.Spec.Names.Plural,
	}
}
