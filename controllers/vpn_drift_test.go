package controllers

import (
	"testing"

	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func driftTestVPN(secret string, env []v1.EnvVar) entanglev1alpha1.VPN {
	return entanglev1alpha1.VPN{
		ObjectMeta: metav1.ObjectMeta{Name: "vpn", Namespace: "ns"},
		Spec: entanglev1alpha1.VPNSpec{
			SecretRef: &secret,
			Env:       env,
		},
	}
}

// liveDaemonset stands in for the DaemonSet as it comes back from the API
// server: the same values, but decoded into fresh allocations.
func liveDaemonset(t *testing.T, r *VPNReconciler, vpn entanglev1alpha1.VPN) *appsv1.DaemonSet {
	t.Helper()

	ds, err := r.genDaemonset(vpn)
	if err != nil {
		t.Fatalf("genDaemonset: %v", err)
	}

	return ds.DeepCopy()
}

func TestPodSpecNeedsUpdateOnVPN(t *testing.T) {
	withEnv := []v1.EnvVar{{Name: "EDGEVPNLOGLEVEL", Value: "info"}}

	for _, tt := range []struct {
		name    string
		env     []v1.EnvVar
		image   string
		mutate  func(vpn *entanglev1alpha1.VPN)
		corrupt func(ds *appsv1.DaemonSet)
		want    bool
	}{
		{name: "unchanged, no env", want: false},
		{name: "unchanged, env set", env: withEnv, want: false},
		{
			name: "secretRef repointed", env: withEnv, want: true,
			mutate: func(vpn *entanglev1alpha1.VPN) {
				other := "rotated-token"
				vpn.Spec.SecretRef = &other
			},
		},
		{
			name: "env value changed", env: withEnv, want: true,
			mutate: func(vpn *entanglev1alpha1.VPN) {
				vpn.Spec.Env = []v1.EnvVar{{Name: "EDGEVPNLOGLEVEL", Value: "debug"}}
			},
		},
		{
			name: "env appended", env: withEnv, want: true,
			mutate: func(vpn *entanglev1alpha1.VPN) {
				vpn.Spec.Env = append(vpn.Spec.Env, v1.EnvVar{Name: "EDGEVPNLIBP2PLOGLEVEL", Value: "debug"})
			},
		},
		{name: "operator image changed", env: withEnv, image: "edgevpn:v2", want: true},
		{
			name: "live env emptied", env: withEnv, want: true,
			corrupt: func(ds *appsv1.DaemonSet) { ds.Spec.Template.Spec.Containers[0].Env = nil },
		},
		{
			name: "live command emptied", env: withEnv, want: true,
			corrupt: func(ds *appsv1.DaemonSet) { ds.Spec.Template.Spec.Containers[0].Command = nil },
		},
		{
			name: "live has no containers", env: withEnv, want: true,
			corrupt: func(ds *appsv1.DaemonSet) { ds.Spec.Template.Spec.Containers = nil },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &VPNReconciler{EntangleServiceImage: "edgevpn:v1"}
			current := liveDaemonset(t, r, driftTestVPN("network-token", tt.env))
			if tt.corrupt != nil {
				tt.corrupt(current)
			}

			vpn := driftTestVPN("network-token", tt.env)
			if tt.mutate != nil {
				tt.mutate(&vpn)
			}
			if tt.image != "" {
				r.EntangleServiceImage = tt.image
			}

			desired, err := r.genDaemonset(vpn)
			if err != nil {
				t.Fatalf("genDaemonset: %v", err)
			}

			got := podSpecNeedsUpdate(desired.Spec.Template.Spec, current.Spec.Template.Spec)
			if got != tt.want {
				t.Fatalf("podSpecNeedsUpdate = %v, want %v", got, tt.want)
			}
		})
	}
}

// The API server hands back a pod template it has defaulted. Those defaults
// must not read as drift, or every reconcile would issue an update.
func TestPodSpecNeedsUpdateIgnoresServerDefaults(t *testing.T) {
	r := &VPNReconciler{EntangleServiceImage: "edgevpn:v1"}
	vpn := driftTestVPN("network-token", []v1.EnvVar{{Name: "EDGEVPNLOGLEVEL", Value: "info"}})

	desired, err := r.genDaemonset(vpn)
	if err != nil {
		t.Fatalf("genDaemonset: %v", err)
	}

	current := desired.DeepCopy()
	hostPathType := v1.HostPathUnset
	current.Spec.Template.Spec.Volumes[0].HostPath.Type = &hostPathType
	current.Spec.Template.Spec.DNSPolicy = v1.DNSClusterFirstWithHostNet
	current.Spec.Template.Spec.RestartPolicy = v1.RestartPolicyAlways
	current.Spec.Template.Spec.SchedulerName = "default-scheduler"
	current.Spec.Template.Spec.Containers[0].TerminationMessagePath = "/dev/termination-log"
	current.Spec.Template.Spec.Containers[0].TerminationMessagePolicy = v1.TerminationMessageReadFile

	if podSpecNeedsUpdate(desired.Spec.Template.Spec, current.Spec.Template.Spec) {
		t.Fatal("podSpecNeedsUpdate reported drift on a pod template the API server only defaulted")
	}
}
