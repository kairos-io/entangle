package controllers

import (
	"testing"

	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// A Service is reachable from another namespace only at its fully qualified
// name, which carries the namespace: <service>.<namespace>.svc.cluster.local.
func TestGenDeploymentServiceRefTarget(t *testing.T) {
	svcName := "my-service"
	secretName := "mysecret"

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: "my-namespace"},
	}

	r := &EntanglementReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(svc).Build(),
	}

	ent := entanglev1alpha1.Entanglement{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "my-namespace"},
		Spec: entanglev1alpha1.EntanglementSpec{
			ServiceUUID: "foo",
			ServiceRef:  &svcName,
			SecretRef:   &secretName,
			Port:        "8080",
		},
	}

	dep, err := r.genDeployment(ent, "info")
	if err != nil {
		t.Fatalf("genDeployment: %v", err)
	}

	args := dep.Spec.Template.Spec.Containers[0].Args
	got := args[len(args)-1]
	want := "my-service.my-namespace.svc.cluster.local:8080"
	if got != want {
		t.Errorf("service target = %q, want %q", got, want)
	}
}
