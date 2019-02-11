package namespace

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/kiwigrid/secret-replicator/pkg/service"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Namespace Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this core.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	var currentNamespace string
	if os.Getenv("SECRET_NAMESPACE") == "" {
		file, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			logf.Log.WithName("pull-secret-controller").Error(err, "")
		}
		currentNamespace = string(file)
	} else {
		currentNamespace = os.Getenv("SECRET_NAMESPACE")
	}

	return &ReconcileNamespace{Client: mgr.GetClient(),
		scheme:            mgr.GetScheme(),
		log:               logf.Log.WithName("pull-namespace-controller"),
		secrets:           strings.Split(os.Getenv("SECRETS_LIST"), ","),
		ignoreNamespaces:  strings.Split(os.Getenv("IGNORE_NAMESPACES"), ","),
		currentNamespace:  currentNamespace,
		PullSecretService: pullsecretservice.NewPullSecretService()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("namespace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Namespace
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileNamespace{}

// ReconcileNamespace reconciles a Namespace object
type ReconcileNamespace struct {
	client.Client
	scheme *runtime.Scheme
	log    logr.Logger
	*pullsecretservice.PullSecretService
	secrets          []string
	ignoreNamespaces []string
	currentNamespace string
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileNamespace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	//ignore current namespace
	if r.currentNamespace == request.Name {
		return reconcile.Result{}, nil
	}

	if contains(r.ignoreNamespaces, request.Name) {
		return reconcile.Result{}, nil
	}

	// Fetch the Namespace instance
	instance := &corev1.Namespace{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// The object is being deleted
	if !instance.ObjectMeta.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil

	}

	secrets := &corev1.SecretList{}
	searchError := r.List(context.TODO(), &client.ListOptions{Namespace: r.currentNamespace}, secrets)
	if searchError != nil {
		r.log.Error(searchError, "ERROR")
	}
	for _, element := range secrets.Items {
		if contains(r.secrets, element.Name) {

			secret := &corev1.Secret{}
			err := r.Get(context.TODO(), types.NamespacedName{Name: element.Name, Namespace: instance.Name}, secret)
			if err != nil {
				if errors.IsNotFound(err) {
					r.log.Info(fmt.Sprintf("secret %s not exists in namespace %s. Create it", element.Name, instance.Name))
					// Object not found, return. Create it
					r.PullSecretService.CreateOrUpdateSecret(r.Client, &element, instance.Name, element.Name)
					continue
				}
			}

		}
	}
	return reconcile.Result{}, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
