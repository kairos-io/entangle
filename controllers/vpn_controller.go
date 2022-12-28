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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
)

// VPNReconciler reconciles a VPN object
type VPNReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	clientSet            *kubernetes.Clientset
	EntangleServiceImage string
}

//+kubebuilder:rbac:groups=entangle.kairos.io,resources=vpns,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=entangle.kairos.io,resources=vpns/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=entangle.kairos.io,resources=vpns/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create;get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VPN object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *VPNReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Creates a deployment targeting a service
	// TODO(user): your logic here
	var ent entanglev1alpha1.VPN
	if err := r.Get(ctx, req.NamespacedName, &ent); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	desiredDaemonset, err := r.genDaemonset(ent)
	if err != nil {
		return ctrl.Result{}, err
	}

	daemonset, err := r.clientSet.AppsV1().DaemonSets(req.Namespace).Get(ctx, desiredDaemonset.Name, v1.GetOptions{})
	if daemonset == nil || apierrors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("Creating Daemonset %v", daemonset))

		daemonset, err = r.clientSet.AppsV1().DaemonSets(req.Namespace).Create(ctx, desiredDaemonset, v1.CreateOptions{})
		if err != nil {
			logger.Error(err, "Failed while creating daemonset")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// // If args or env are missing, update it
	// if desiredDaemonset.{
	// 	deployment, err = r.clientSet.AppsV1().Deployments(req.Namespace).Update(ctx, desiredDeployment, v1.UpdateOptions{})
	// 	if err != nil {
	// 		logger.Error(err, "Failed while updating deployment")
	// 		return ctrl.Result{}, nil
	// 	}
	// }

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VPNReconciler) SetupWithManager(mgr ctrl.Manager) error {
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	r.clientSet = clientset
	return ctrl.NewControllerManagedBy(mgr).
		For(&entanglev1alpha1.VPN{}).
		Complete(r)
}
