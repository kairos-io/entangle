package controllers

import (
	"testing"

	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// secretRef is optional in the CRD, so both generators have to report it
// missing instead of dereferencing a nil pointer and taking the manager down.
func TestGenDeploymentWithoutSecretRef(t *testing.T) {
	r := &EntanglementReconciler{}
	ent := entanglev1alpha1.Entanglement{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: entanglev1alpha1.EntanglementSpec{
			ServiceUUID: "foo",
			Host:        "127.0.0.1",
			Port:        "8080",
		},
	}

	if _, err := r.genDeployment(ent, "info"); err == nil {
		t.Fatal("genDeployment accepted an Entanglement with no secretRef")
	}
}

func TestGenDaemonsetWithoutSecretRef(t *testing.T) {
	r := &VPNReconciler{}
	ent := entanglev1alpha1.VPN{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	if _, err := r.genDaemonset(ent); err == nil {
		t.Fatal("genDaemonset accepted a VPN with no secretRef")
	}
}
