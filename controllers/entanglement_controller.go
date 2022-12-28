/*
Copyright 2022.

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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
)

// EntanglementReconciler reconciles a Entanglement object
type EntanglementReconciler struct {
	clientSet *kubernetes.Clientset
	client.Client
	Scheme                         *runtime.Scheme
	EntangleServiceImage, LogLevel string
}

//+kubebuilder:rbac:groups=entangle.kairos.io,resources=entanglements,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=entangle.kairos.io,resources=entanglements/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=entangle.kairos.io,resources=entanglements/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=create;get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Entanglement object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *EntanglementReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Creates a deployment targeting a service
	// TODO(user): your logic here
	var ent entanglev1alpha1.Entanglement
	if err := r.Get(ctx, req.NamespacedName, &ent); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	desiredDeployment, err := r.genDeployment(ent, r.LogLevel)
	if err != nil {
		return ctrl.Result{}, err
	}

	if ent.Spec.Inbound {
		if ent.Spec.ServiceSpec == nil {
			return ctrl.Result{Requeue: false}, fmt.Errorf("servicespec is required with an inbound connection")
		}
		svc := genService(ent)

		sv, err := r.clientSet.CoreV1().Services(req.Namespace).Get(ctx, svc.Name, v1.GetOptions{})
		if sv == nil || apierrors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Creating Service %v", sv))

			sv, err = r.clientSet.CoreV1().Services(req.Namespace).Create(ctx, svc, v1.CreateOptions{})
			if err != nil {
				logger.Error(err, "Failed while creating deployment")
				return ctrl.Result{}, nil
			}

			return ctrl.Result{Requeue: true}, nil
		}
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	deployment, err := r.clientSet.AppsV1().Deployments(req.Namespace).Get(ctx, desiredDeployment.Name, v1.GetOptions{})
	if deployment == nil || apierrors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("Creating Deployment %v", deployment))

		deployment, err = r.clientSet.AppsV1().Deployments(req.Namespace).Create(ctx, desiredDeployment, v1.CreateOptions{})
		if err != nil {
			logger.Error(err, "Failed while creating deployment")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// If args or env are missing, update it
	if desiredDeployment.Spec.Template.Spec.Containers[0].Args[0] != deployment.Spec.Template.Spec.Containers[0].Args[0] ||
		desiredDeployment.Spec.Template.Spec.Containers[0].Env[0] != deployment.Spec.Template.Spec.Containers[0].Env[0] {
		deployment, err = r.clientSet.AppsV1().Deployments(req.Namespace).Update(ctx, desiredDeployment, v1.UpdateOptions{})
		if err != nil {
			logger.Error(err, "Failed while updating deployment")
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EntanglementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	r.clientSet = clientset

	return ctrl.NewControllerManagedBy(mgr).
		For(&entanglev1alpha1.Entanglement{}).
		Complete(r)
}

func genService(ent entanglev1alpha1.Entanglement) *corev1.Service {
	objMeta := metav1.ObjectMeta{
		Name:            ent.Name,
		Namespace:       ent.Namespace,
		OwnerReferences: genOwner(ent),
	}

	svc := ent.Spec.ServiceSpec
	svc.Selector = genDeploymentLabel(ent.Name)
	return &corev1.Service{
		ObjectMeta: objMeta,
		Spec:       *svc,
	}
}
