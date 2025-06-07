// Copyright 2025 The Kube Resource Orchestrator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"context"

	krov1alpha1 "github.com/kro-run/kro/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/scale/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getKubeClient() (client.Client, error) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = krov1alpha1.AddToScheme(s)
	cfg := ctrl.GetConfigOrDie()
	return client.New(cfg, client.Options{Scheme: s})
}

func getRGDIfExists(c client.Client, name string) (krov1alpha1.ResourceGraphDefinition, bool, error) {
	var rgd krov1alpha1.ResourceGraphDefinition
	err := c.Get(context.Background(), client.ObjectKey{Name: name}, &rgd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return krov1alpha1.ResourceGraphDefinition{}, false, nil
		}
		return krov1alpha1.ResourceGraphDefinition{}, false, err
	}
	return rgd, true, nil
}
