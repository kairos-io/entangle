package controllers

import (
	"testing"

	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func drifTestEntanglement(envs []v1.EnvVar) entanglev1alpha1.Entanglement {
	secret := "network-token"
	return entanglev1alpha1.Entanglement{
		ObjectMeta: metav1.ObjectMeta{Name: "ent", Namespace: "ns"},
		Spec: entanglev1alpha1.EntanglementSpec{
			SecretRef:   &secret,
			ServiceUUID: "uuid",
			Host:        "10.0.0.1",
			Port:        "8080",
			Envs:        envs,
		},
	}
}

// live stands in for the Deployment as it comes back from the API server: the
// same values, but decoded into fresh allocations.
func live(t *testing.T, r *EntanglementReconciler, ent entanglev1alpha1.Entanglement) *appsv1.Deployment {
	t.Helper()

	d, err := r.genDeployment(ent, "info")
	if err != nil {
		t.Fatalf("genDeployment: %v", err)
	}

	return d.DeepCopy()
}

func TestDeploymentNeedsUpdate(t *testing.T) {
	withEnvs := []v1.EnvVar{{Name: "FOO", Value: "bar"}}

	for _, tt := range []struct {
		name    string
		envs    []v1.EnvVar
		image   string
		mutate  func(ent *entanglev1alpha1.Entanglement)
		corrupt func(d *appsv1.Deployment)
		want    bool
	}{
		{name: "unchanged, no envs", want: false},
		{name: "unchanged, envs set", envs: withEnvs, want: false},
		{
			name: "host changed", envs: withEnvs, want: true,
			mutate: func(ent *entanglev1alpha1.Entanglement) { ent.Spec.Host = "10.0.0.99" },
		},
		{
			name: "port changed", envs: withEnvs, want: true,
			mutate: func(ent *entanglev1alpha1.Entanglement) { ent.Spec.Port = "9090" },
		},
		{
			name: "serviceUUID changed", envs: withEnvs, want: true,
			mutate: func(ent *entanglev1alpha1.Entanglement) { ent.Spec.ServiceUUID = "other" },
		},
		{
			name: "inbound flipped", envs: withEnvs, want: true,
			mutate: func(ent *entanglev1alpha1.Entanglement) { ent.Spec.Inbound = true },
		},
		{
			name: "hostNetwork flipped", envs: withEnvs, want: true,
			mutate: func(ent *entanglev1alpha1.Entanglement) { ent.Spec.HostNetwork = true },
		},
		{
			name: "env appended", envs: withEnvs, want: true,
			mutate: func(ent *entanglev1alpha1.Entanglement) {
				ent.Spec.Envs = append(ent.Spec.Envs, v1.EnvVar{Name: "BAZ", Value: "qux"})
			},
		},
		{name: "sidecar image changed", envs: withEnvs, image: "entangle:v2", want: true},
		{
			name: "live args emptied", envs: withEnvs, want: true,
			corrupt: func(d *appsv1.Deployment) { d.Spec.Template.Spec.Containers[0].Args = nil },
		},
		{
			name: "live env emptied", envs: withEnvs, want: true,
			corrupt: func(d *appsv1.Deployment) { d.Spec.Template.Spec.Containers[0].Env = nil },
		},
		{
			name: "live has no containers", envs: withEnvs, want: true,
			corrupt: func(d *appsv1.Deployment) { d.Spec.Template.Spec.Containers = nil },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &EntanglementReconciler{EntangleServiceImage: "entangle:v1"}
			current := live(t, r, drifTestEntanglement(tt.envs))
			if tt.corrupt != nil {
				tt.corrupt(current)
			}

			ent := drifTestEntanglement(tt.envs)
			if tt.mutate != nil {
				tt.mutate(&ent)
			}
			if tt.image != "" {
				r = &EntanglementReconciler{EntangleServiceImage: tt.image}
			}

			desired, err := r.genDeployment(ent, "info")
			if err != nil {
				t.Fatalf("genDeployment: %v", err)
			}

			if got := deploymentNeedsUpdate(desired, current); got != tt.want {
				t.Errorf("deploymentNeedsUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}
