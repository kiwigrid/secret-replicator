/*

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

package controllers

import (
	"context"
	"fmt"
	"regexp"

	"github.com/go-logr/logr"
	"github.com/kiwigrid/secret-replicator/service"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Log logr.Logger
	*service.PullSecretService
	Secrets           []string
	IgnoreNamespaces  []string
	IncludeNamespaces []string
	CurrentNamespace  string
}

// +kubebuilder:rbac:groups=secret.kiwigrid.com,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secret.kiwigrid.com,resources=secrets/status,verbs=get;update;patch

func (r *SecretReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("secret", req.NamespacedName)

	// your logic here

	// only secrets from lookup namespace
	if req.Namespace != r.CurrentNamespace {
		return reconcile.Result{}, nil
	}

	// only pull secrets
	if !contains(r.Secrets, req.Name) {
		return reconcile.Result{}, nil
	}

	instance := &v1.Secret{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	namespaces := &v1.NamespaceList{}
	searchError := r.List(ctx, namespaces) //, client.InNamespace(""))
	if searchError != nil {
		r.Log.Error(searchError, "ERROR")
	}
	for _, element := range namespaces.Items {
		if contains(r.IgnoreNamespaces, element.Name) {
			continue
		}
		// only use include namespaces if it is not empty
		if len(r.IncludeNamespaces) > 0 {
			if contains(r.IncludeNamespaces, element.Name) {
				r.Log.Info(fmt.Sprintf("Create or update secret %s in namespace %s", instance.Name, element.Name))
				r.PullSecretService.CreateOrUpdateSecret(r.Client, instance, element.Name, instance.Name)
			}
		} else {
			r.Log.Info(fmt.Sprintf("Create or update secret %s in namespace %s", instance.Name, element.Name))
			r.PullSecretService.CreateOrUpdateSecret(r.Client, instance, element.Name, instance.Name)
		}
	}
	return ctrl.Result{}, nil
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Secret{}).
		Complete(r)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		matched, _ := regexp.Match(a, []byte(e))
		if a == e || matched {
			return true
		}
	}
	return false
}
