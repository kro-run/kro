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
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	testScheme   = runtime.NewScheme()
	configMapGVK = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}
	secretGVK    = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
)

func init() {
	testScheme.AddKnownTypes(configMapGVK.GroupVersion(), &unstructured.Unstructured{})
	testScheme.AddKnownTypes(secretGVK.GroupVersion(), &unstructured.Unstructured{})
}

func newTestApplySet(t *testing.T, parent *unstructured.Unstructured, config Config, objs ...runtime.Object) (Set, *fake.FakeDynamicClient) {
	allObjs := append([]runtime.Object{parent}, objs...)
	// The fake client needs to know about the resources we are going to interact with
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(testScheme, map[schema.GroupVersionResource]string{
		gvr: "ConfigMapList",
	}, allObjs...)

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{parent.GroupVersionKind().GroupVersion(), configMapGVK.GroupVersion()})
	restMapper.Add(parent.GroupVersionKind(), meta.RESTScopeNamespace)
	restMapper.Add(configMapGVK, meta.RESTScopeNamespace)

	if config.Log == (logr.Logger{}) {
		config.Log = logr.Discard()
	}

	aset, err := New(parent, restMapper, dynamicClient, config)
	assert.NoError(t, err)
	return aset, dynamicClient
}

func TestNew(t *testing.T) {
	parent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "parent-secret",
				"namespace": "default",
			},
		},
	}
	parent.SetGroupVersionKind(secretGVK)

	t.Run("valid config", func(t *testing.T) {
		_, _ = newTestApplySet(t, parent, Config{
			ToolingID:    ToolingID{Name: "test", Version: "v1"},
			FieldManager: "test-manager",
		})
	})

	t.Run("missing toolingID", func(t *testing.T) {
		_, err := New(parent, nil, nil, Config{
			FieldManager: "test-manager",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "toolingID is required")
	})

	t.Run("missing fieldManager", func(t *testing.T) {
		_, err := New(parent, nil, nil, Config{
			ToolingID: ToolingID{Name: "test", Version: "v1"},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fieldManager is required")
	})
}

func TestApplySet_Add(t *testing.T) {
	parent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "parent-secret",
				"namespace": "default",
				"uid":       "parent-uid",
			},
		},
	}
	parent.SetGroupVersionKind(secretGVK)

	aset, _ := newTestApplySet(t, parent, Config{
		ToolingID:    ToolingID{Name: "test", Version: "v1"},
		FieldManager: "test-manager",
	})

	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}
	cm.SetGroupVersionKind(configMapGVK)

	applyableObj := ApplyableObject{
		Unstructured: cm,
		ID:           "cm1",
	}

	err := aset.Add(context.Background(), applyableObj)
	assert.NoError(t, err)

	as := aset.(*applySet)
	assert.Equal(t, 1, as.desired.Len())
	assert.Contains(t, as.desired.objects[0].GetLabels(), ApplysetPartOfLabel)

	assert.True(t, as.desiredNamespaces.Has("default"))
	_, exists := as.desiredRESTMappings[cm.GroupVersionKind().GroupKind()]
	assert.True(t, exists)
}

func TestApplySet_ID(t *testing.T) {
	parent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "foo.bar/v1",
			"kind":       "Foo",
			"metadata": map[string]interface{}{
				"name":      "test-foo",
				"namespace": "test-ns",
			},
		},
	}
	parent.SetGroupVersionKind(schema.GroupVersionKind{Group: "foo.bar", Version: "v1", Kind: "Foo"})

	// We need a fake client and mapper that know about this custom type
	dynamicClient := fake.NewSimpleDynamicClient(testScheme, parent)
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{parent.GroupVersionKind().GroupVersion()})
	restMapper.Add(parent.GroupVersionKind(), meta.RESTScopeNamespace)

	aset, err := New(parent, restMapper, dynamicClient, Config{
		ToolingID:    ToolingID{Name: "test", Version: "v1"},
		FieldManager: "test-manager",
		Log:          logr.Discard(),
	})
	assert.NoError(t, err)

	// from: base64(sha256(<name>.<namespace>.<kind>.<group>))
	// test-foo.test-ns.Foo.foo.bar
	// sha256: 2dca1c8242de82132464575841648a37c74545785854614378933d2d408ddd2b
	// base64: LcocgkLeghMkZFdYQWSKN8dFRXhYVGFDeJM9LUCd3Ss
	// format: applyset-%s-v1
	expectedID := "applyset-f9Rk5tKHoB72oV1tU2iFKxDwL8MBZTjMHFQ8V9WNNlA-v1"
	assert.Equal(t, expectedID, aset.(*applySet).ID())
}

func TestApplySet_Apply(t *testing.T) {
	parent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "parent-secret",
				"namespace": "default",
				"uid":       "parent-uid",
			},
		},
	}
	parent.SetGroupVersionKind(secretGVK)

	aset, dynamicClient := newTestApplySet(t, parent, Config{
		ToolingID:    ToolingID{Name: "test", Version: "v1"},
		FieldManager: "test-manager",
	})

	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}
	cm.SetGroupVersionKind(configMapGVK)

	err := aset.Add(context.Background(), ApplyableObject{Unstructured: cm, ID: "cm1"})
	assert.NoError(t, err)

	var appliedCM *unstructured.Unstructured
	dynamicClient.PrependReactor("patch", "configmaps", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(k8stesting.PatchAction)
		assert.Equal(t, types.ApplyPatchType, patchAction.GetPatchType())

		err = json.Unmarshal(patchAction.GetPatch(), &appliedCM)
		assert.NoError(t, err)

		// The fake client needs to return the object that was applied.
		return true, appliedCM, nil
	})
	// Reactor for parent update
	dynamicClient.PrependReactor("patch", "secrets", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(k8stesting.PatchAction)
		assert.Equal(t, types.ApplyPatchType, patchAction.GetPatchType())

		var parentPatch unstructured.Unstructured
		err = json.Unmarshal(patchAction.GetPatch(), &parentPatch)
		assert.NoError(t, err)

		return true, &parentPatch, nil
	})

	result, err := aset.Apply(context.Background(), false)
	assert.NoError(t, err)
	assert.NoError(t, result.Errors())
	assert.Equal(t, 1, result.DesiredCount)
	assert.Len(t, result.AppliedObjects, 1)
	assert.NotNil(t, appliedCM)
	assert.Equal(t, "test-cm", appliedCM.GetName())
	assert.Contains(t, appliedCM.GetLabels(), ApplysetPartOfLabel)
}
