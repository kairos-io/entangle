package controllers

import (
	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func genDaemonsetOwner(ent entanglev1alpha1.VPN) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		*metav1.NewControllerRef(&ent.ObjectMeta, schema.GroupVersionKind{
			Group:   entanglev1alpha1.GroupVersion.Group,
			Version: entanglev1alpha1.GroupVersion.Version,
			Kind:    "VPN",
		}),
	}
}

func (r *VPNReconciler) genDaemonset(ent entanglev1alpha1.VPN) (*appsv1.DaemonSet, error) {
	objMeta := metav1.ObjectMeta{
		Name:            ent.Name,
		Namespace:       ent.Namespace,
		OwnerReferences: genDaemonsetOwner(ent),
	}

	privileged := true
	serviceAccount := false

	v := ent.Spec.Env
	v = append(v, v1.EnvVar{
		Name: "EDGEVPNTOKEN",
		ValueFrom: &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				Key: "network_token",
				LocalObjectReference: v1.LocalObjectReference{
					Name: *ent.Spec.SecretRef,
				},
			},
		},
	})

	expose := v1.Container{
		SecurityContext: &v1.SecurityContext{
			Privileged: &privileged,
			Capabilities: &v1.Capabilities{
				Add: []v1.Capability{"NET_ADMIN"},
			},
		},
		ImagePullPolicy: v1.PullAlways,
		Name:            "vpn",
		Image:           r.EntangleServiceImage,
		Env:             v,
		Command:         []string{"/usr/bin/edgevpn"},
		VolumeMounts:    []v1.VolumeMount{v1.VolumeMount{Name: "dev-net-tun", MountPath: "/dev/net/tun"}},
	}

	pod := v1.PodSpec{
		Containers:                   []v1.Container{expose},
		AutomountServiceAccountToken: &serviceAccount,
		HostNetwork:                  true,
		Volumes:                      []v1.Volume{v1.Volume{Name: "dev-net-tun", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev/net/tun"}}}},
	}

	deploymentLabels := getnDaemonsetLabel(ent.Name)

	return &appsv1.DaemonSet{
		ObjectMeta: objMeta,

		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: deploymentLabels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: deploymentLabels,
				},
				Spec: pod,
			},
		},
	}, nil
}

func getnDaemonsetLabel(s string) map[string]string {
	return map[string]string{
		"vpn.kairos.io": s,
	}
}
