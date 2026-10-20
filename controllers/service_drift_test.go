package controllers

import (
	"context"
	"testing"

	entanglev1alpha1 "github.com/kairos-io/entangle/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	crfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func inboundEntanglement(port int32) *entanglev1alpha1.Entanglement {
	secret := "network-token"
	return &entanglev1alpha1.Entanglement{
		ObjectMeta: metav1.ObjectMeta{Name: "ent", Namespace: "ns"},
		Spec: entanglev1alpha1.EntanglementSpec{
			SecretRef:   &secret,
			ServiceUUID: "uuid",
			Host:        "10.0.0.1",
			Port:        "8080",
			Inbound:     true,
			ServiceSpec: &corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Name: "http", Port: port}},
			},
		},
	}
}

// TestServiceIsReconciledOnDrift edits the serviceSpec of an inbound
// Entanglement whose Service already exists, and expects the live Service to
// follow.
func TestServiceIsReconciledOnDrift(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := entanglev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	// The Service as it was created on the first reconcile, when the spec still
	// asked for port 80. The API server fills in a ClusterIP.
	existing := genService(*inboundEntanglement(80))
	existing.Spec.ClusterIP = "10.96.0.10"

	// The Entanglement now asks for port 8443.
	ent := inboundEntanglement(8443)

	r := &EntanglementReconciler{
		clientSet:            k8sfake.NewSimpleClientset(existing),
		Client:               crfake.NewClientBuilder().WithScheme(scheme).WithObjects(ent).Build(),
		Scheme:               scheme,
		EntangleServiceImage: "entangle:v1",
		LogLevel:             "info",
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "ent", Namespace: "ns"}}
	for i := 0; i < 3; i++ {
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
	}

	live, err := r.clientSet.CoreV1().Services("ns").Get(context.Background(), "ent", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get service: %v", err)
	}

	if got := live.Spec.Ports[0].Port; got != 8443 {
		t.Fatalf("service port = %d, want 8443 (the Service never followed the spec)", got)
	}
	if live.Spec.ClusterIP != "10.96.0.10" {
		t.Fatalf("clusterIP = %q, want it preserved: it is immutable", live.Spec.ClusterIP)
	}
}

// TestServiceIsNotUpdatedWhenItMatches guards the other direction: the API
// server defaults a Service far beyond what the user wrote, and the reconciler
// must not read that defaulting as drift and update on every pass.
func TestServiceIsNotUpdatedWhenItMatches(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := entanglev1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	ent := inboundEntanglement(80)

	// The Service as the API server hands it back: the user's two fields plus
	// everything it defaulted.
	existing := genService(*inboundEntanglement(80))
	existing.Spec.ClusterIP = "10.96.0.10"
	existing.Spec.ClusterIPs = []string{"10.96.0.10"}
	existing.Spec.SessionAffinity = corev1.ServiceAffinityNone
	existing.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
	existing.Spec.Ports[0].Protocol = corev1.ProtocolTCP
	existing.Spec.Ports[0].TargetPort = intstr.FromInt(80)

	clientSet := k8sfake.NewSimpleClientset(existing)
	r := &EntanglementReconciler{
		clientSet:            clientSet,
		Client:               crfake.NewClientBuilder().WithScheme(scheme).WithObjects(ent).Build(),
		Scheme:               scheme,
		EntangleServiceImage: "entangle:v1",
		LogLevel:             "info",
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "ent", Namespace: "ns"}}
	for i := 0; i < 3; i++ {
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
	}

	for _, a := range clientSet.Actions() {
		if a.GetResource().Resource == "services" && a.GetVerb() == "update" {
			t.Fatalf("reconciler updated a Service that already matches the spec")
		}
	}
}

func TestMergeServicePortsKeepsTheAllocatedNodePort(t *testing.T) {
	live := []corev1.ServicePort{
		{Name: "http", Port: 80, NodePort: 31234},
		{Name: "metrics", Port: 9090, NodePort: 31235},
	}

	out := mergeServicePorts([]corev1.ServicePort{
		{Name: "http", Port: 8443},                     // retargeted, node port not pinned
		{Name: "metrics", Port: 9090, NodePort: 32000}, // node port pinned by the user
	}, live)

	if len(out) != 2 {
		t.Fatalf("got %d ports, want 2", len(out))
	}
	if out[0].NodePort != 31234 {
		t.Errorf("http nodePort = %d, want the allocated 31234", out[0].NodePort)
	}
	if out[0].Port != 8443 {
		t.Errorf("http port = %d, want 8443", out[0].Port)
	}
	if out[1].NodePort != 32000 {
		t.Errorf("metrics nodePort = %d, want the pinned 32000", out[1].NodePort)
	}
}

func TestMergeServicePortsDropsARemovedPort(t *testing.T) {
	live := []corev1.ServicePort{
		{Name: "http", Port: 80, NodePort: 31234},
		{Name: "metrics", Port: 9090, NodePort: 31235},
	}

	out := mergeServicePorts([]corev1.ServicePort{{Name: "http", Port: 80}}, live)
	if len(out) != 1 || out[0].Name != "http" {
		t.Fatalf("got %+v, want only the http port", out)
	}
}

func TestReconciledServiceNeverMovesTheClusterIP(t *testing.T) {
	live := &corev1.Service{Spec: corev1.ServiceSpec{
		ClusterIP:  "10.96.0.10",
		ClusterIPs: []string{"10.96.0.10"},
		Type:       corev1.ServiceTypeClusterIP,
		Ports:      []corev1.ServicePort{{Name: "http", Port: 80}},
	}}
	desired := &corev1.Service{Spec: corev1.ServiceSpec{
		Type:  corev1.ServiceTypeNodePort,
		Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
	}}

	out, changed := reconciledService(desired, live)
	if !changed {
		t.Fatal("a service type change is drift")
	}
	if out.Spec.ClusterIP != "10.96.0.10" || len(out.Spec.ClusterIPs) != 1 {
		t.Fatalf("clusterIP not preserved: %+v", out.Spec)
	}
	if out.Spec.Type != corev1.ServiceTypeNodePort {
		t.Fatalf("type = %q, want NodePort", out.Spec.Type)
	}
}

// A named port carries the targetPort the API server defaulted from the old
// port number. Moving the port must not keep that stale value, or traffic goes
// to the port the user moved away from.
func TestMergeServicePortsDropsATargetPortDefaultedFromTheOldPort(t *testing.T) {
	live := []corev1.ServicePort{
		{Name: "http", Port: 80, TargetPort: intstr.FromInt(80), NodePort: 31234},
	}

	out := mergeServicePorts([]corev1.ServicePort{{Name: "http", Port: 9000}}, live)

	if len(out) != 1 {
		t.Fatalf("got %d ports, want 1", len(out))
	}
	if out[0].TargetPort != (intstr.IntOrString{}) {
		t.Errorf("targetPort = %v, want it left empty so the API server defaults it to 9000", out[0].TargetPort)
	}
	if out[0].NodePort != 31234 {
		t.Errorf("nodePort = %d, want the allocated 31234 kept", out[0].NodePort)
	}
}

// The user's own targetPort is desired, not live, so a port move keeps it.
func TestMergeServicePortsKeepsAnExplicitTargetPortAcrossAPortMove(t *testing.T) {
	live := []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(80)}}

	out := mergeServicePorts([]corev1.ServicePort{
		{Name: "http", Port: 9000, TargetPort: intstr.FromString("web")},
	}, live)

	if out[0].TargetPort != intstr.FromString("web") {
		t.Errorf("targetPort = %v, want the spec's own \"web\"", out[0].TargetPort)
	}
}

// While the port is unchanged the live targetPort is still what the API server
// filled in for this port, so it is carried over and the merge stays a no-op.
func TestMergeServicePortsKeepsTheTargetPortWhileThePortIsUnchanged(t *testing.T) {
	live := []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)}}

	out := mergeServicePorts([]corev1.ServicePort{{Name: "http", Port: 80}}, live)

	if out[0].TargetPort != intstr.FromInt(8080) {
		t.Errorf("targetPort = %v, want the live 8080 kept", out[0].TargetPort)
	}
}

// Dropping `type: NodePort` from the spec is not "no opinion": the API server
// defaults an empty type to ClusterIP on create, so the same spec applied to a
// live NodePort Service has to take it back to ClusterIP.
func TestReconciledServiceTakesAnEmptyTypeBackToClusterIP(t *testing.T) {
	live := &corev1.Service{Spec: corev1.ServiceSpec{
		ClusterIP:             "10.96.0.10",
		ClusterIPs:            []string{"10.96.0.10"},
		Type:                  corev1.ServiceTypeNodePort,
		ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeCluster,
		Ports: []corev1.ServicePort{
			{Name: "http", Port: 9000, TargetPort: intstr.FromInt(9000), NodePort: 30818},
		},
	}}
	desired := &corev1.Service{Spec: corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{Name: "http", Port: 9000}},
	}}

	out, changed := reconciledService(desired, live)
	if !changed {
		t.Fatal("dropping the type is drift: the Service is still on NodePort")
	}
	if out.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatalf("type = %q, want ClusterIP, what the same spec gets on create", out.Spec.Type)
	}
	if out.Spec.ClusterIP != "10.96.0.10" {
		t.Fatalf("clusterIP = %q, want it preserved: it is immutable", out.Spec.ClusterIP)
	}
	// The node port and the traffic policy go out unchanged on purpose. The API
	// server drops both when the type stops needing them and the values did not
	// move, and sending a cleared nodePort with a kept one elsewhere is how a
	// node port gets reallocated by accident.
	if out.Spec.Ports[0].NodePort != 30818 {
		t.Errorf("nodePort = %d, want the allocated 30818 sent back unchanged", out.Spec.Ports[0].NodePort)
	}
}

// The switch has to settle: once the Service is ClusterIP, a spec with no type
// must read as a match, or the reconciler updates on every pass.
func TestReconciledServiceWithAnEmptyTypeSettlesOnClusterIP(t *testing.T) {
	live := &corev1.Service{Spec: corev1.ServiceSpec{
		ClusterIP:  "10.96.0.10",
		ClusterIPs: []string{"10.96.0.10"},
		Type:       corev1.ServiceTypeClusterIP,
		Ports: []corev1.ServicePort{
			{Name: "http", Port: 9000, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(9000)},
		},
	}}
	desired := &corev1.Service{Spec: corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{Name: "http", Port: 9000}},
	}}

	if _, changed := reconciledService(desired, live); changed {
		t.Fatal("a ClusterIP Service already matches a spec that names no type")
	}
}
