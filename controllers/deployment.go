package controllers

import (
	"context"
	"fmt"

	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func genOwner(ent entanglev1alpha1.Entanglement) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		*metav1.NewControllerRef(&ent.ObjectMeta, schema.GroupVersionKind{
			Group:   entanglev1alpha1.GroupVersion.Group,
			Version: entanglev1alpha1.GroupVersion.Version,
			Kind:    "Entanglement",
		}),
	}
}

func (r *EntanglementReconciler) genDeployment(ent entanglev1alpha1.Entanglement, logLevel string) (*appsv1.Deployment, error) {
	objMeta := metav1.ObjectMeta{
		Name:            ent.Name,
		Namespace:       ent.Namespace,
		OwnerReferences: genOwner(ent),
	}

	privileged := false
	serviceAccount := false

	svc := &v1.Service{}
	if ent.Spec.ServiceRef != nil {
		err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: ent.Namespace, Name: *ent.Spec.ServiceRef}, svc)
		if err != nil {
			return nil, err
		}
	}

	expose := v1.Container{
		ImagePullPolicy: v1.PullAlways,
		SecurityContext: &v1.SecurityContext{Privileged: &privileged},
		Name:            "entanglement",
		Image:           r.EntangleServiceImage,
		Env: []v1.EnvVar{
			{
				Name: "EDGEVPNTOKEN",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						Key: "network_token",
						LocalObjectReference: v1.LocalObjectReference{
							Name: *ent.Spec.SecretRef,
						},
					},
				},
			},
		},
		Command: []string{"/usr/bin/edgevpn"},
	}

	cmd := "service-add"
	if ent.Spec.Inbound {
		// p, err := strconv.Atoi(ent.Spec.Port)
		// if err != nil {
		// 	return nil, err
		// }
		cmd = "service-connect"
		// expose.Ports = []v1.ContainerPort{
		// 	{

		// 		Name:          "service",
		// 		ContainerPort: int32(p),
		// 	},
		// }
		// expose.ReadinessProbe = &v1.Probe{
		// 	ProbeHandler: v1.ProbeHandler{
		// 		Exec: &v1.ExecAction{
		// 			Command: []string{"/bin/bash", "-xce", ""},
		// 		},
		// 	},
		// 	InitialDelaySeconds: 60,
		// 	PeriodSeconds:       30,
		// 	SuccessThreshold:    3,
		// 	FailureThreshold:    3,
		// }
		// expose.LivenessProbe = &v1.Probe{
		// 	ProbeHandler:        v1.ProbeHandler{},
		// 	InitialDelaySeconds: 220,
		// 	PeriodSeconds:       120,
		// 	SuccessThreshold:    1,
		// 	FailureThreshold:    3,
		// }
	}

	if ent.Spec.ServiceRef != nil {
		expose.Args = []string{cmd, "--log-level", logLevel, ent.Spec.ServiceUUID, fmt.Sprintf("%s:%s", fmt.Sprintf("%s.svc.cluster.local", svc.Name), ent.Spec.Port)}
	} else {
		expose.Args = []string{cmd, "--log-level", logLevel, ent.Spec.ServiceUUID, fmt.Sprintf("%s:%s", ent.Spec.Host, ent.Spec.Port)}
	}

	pod := v1.PodSpec{
		Containers:                   []v1.Container{expose},
		AutomountServiceAccountToken: &serviceAccount,
		HostNetwork:                  ent.Spec.HostNetwork,
	}

	deploymentLabels := genDeploymentLabel(ent.Name)
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: objMeta,

		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: deploymentLabels},
			Replicas: &replicas,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: deploymentLabels,
				},
				Spec: pod,
			},
		},
	}, nil
}

func genDeploymentLabel(s string) map[string]string {
	return map[string]string{
		"entanglement.kairos.io": s,
	}
}
