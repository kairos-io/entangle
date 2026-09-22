package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// reconciledService returns the Service to send back to the API server so that
// live carries the configuration the Entanglement asks for, and reports whether
// anything actually changed.
//
// It is a merge and not a replacement for two reasons. The API server assigns
// ClusterIP, node ports and the IP families itself, and refuses an update that
// changes them; and the user's serviceSpec is a partial object, so a field left
// at its zero value means "no opinion", not "clear it". Merging both ways round
// also keeps the comparison stable: after one update the merge is a no-op, so
// the reconciler does not fight the API server's defaulting in a loop.
func reconciledService(desired, live *corev1.Service) (*corev1.Service, bool) {
	out := live.DeepCopy()
	out.Spec = mergeServiceSpec(desired.Spec, live.Spec)

	return out, !equality.Semantic.DeepEqual(out.Spec, live.Spec)
}

// mergeServiceSpec overlays the fields the Entanglement sets onto the live spec.
// ClusterIP and ClusterIPs are never taken from desired: they are immutable and
// only the API server may pick them.
func mergeServiceSpec(desired, live corev1.ServiceSpec) corev1.ServiceSpec {
	out := *live.DeepCopy()

	if len(desired.Ports) > 0 {
		out.Ports = mergeServicePorts(desired.Ports, live.Ports)
	}
	if len(desired.Selector) > 0 {
		out.Selector = desired.Selector
	}
	if desired.Type != "" {
		out.Type = desired.Type
	}
	if len(desired.ExternalIPs) > 0 {
		out.ExternalIPs = desired.ExternalIPs
	}
	if desired.ExternalName != "" {
		out.ExternalName = desired.ExternalName
	}
	if desired.SessionAffinity != "" {
		out.SessionAffinity = desired.SessionAffinity
	}
	if desired.SessionAffinityConfig != nil {
		out.SessionAffinityConfig = desired.SessionAffinityConfig
	}
	if desired.ExternalTrafficPolicy != "" {
		out.ExternalTrafficPolicy = desired.ExternalTrafficPolicy
	}
	if desired.InternalTrafficPolicy != nil {
		out.InternalTrafficPolicy = desired.InternalTrafficPolicy
	}
	if desired.HealthCheckNodePort != 0 {
		out.HealthCheckNodePort = desired.HealthCheckNodePort
	}
	if desired.PublishNotReadyAddresses {
		out.PublishNotReadyAddresses = true
	}
	if desired.LoadBalancerIP != "" {
		out.LoadBalancerIP = desired.LoadBalancerIP
	}
	if desired.LoadBalancerClass != nil {
		out.LoadBalancerClass = desired.LoadBalancerClass
	}
	if len(desired.LoadBalancerSourceRanges) > 0 {
		out.LoadBalancerSourceRanges = desired.LoadBalancerSourceRanges
	}
	if desired.AllocateLoadBalancerNodePorts != nil {
		out.AllocateLoadBalancerNodePorts = desired.AllocateLoadBalancerNodePorts
	}
	if len(desired.IPFamilies) > 0 {
		out.IPFamilies = desired.IPFamilies
	}
	if desired.IPFamilyPolicy != nil {
		out.IPFamilyPolicy = desired.IPFamilyPolicy
	}

	return out
}

// mergeServicePorts replaces the port list wholesale, because a removed port has
// to disappear, but carries over what the API server filled in on a port the
// user left blank: the allocated node port, and the protocol, target port and
// application protocol it defaults. Ports are matched by name, and by port
// number for the single-port case where the name is empty.
func mergeServicePorts(desired, live []corev1.ServicePort) []corev1.ServicePort {
	out := make([]corev1.ServicePort, len(desired))
	copy(out, desired)

	for i := range out {
		l, ok := findServicePort(live, out[i])
		if !ok {
			continue
		}
		if out[i].NodePort == 0 {
			out[i].NodePort = l.NodePort
		}
		if out[i].Protocol == "" {
			out[i].Protocol = l.Protocol
		}
		if out[i].TargetPort == (intstr.IntOrString{}) {
			out[i].TargetPort = l.TargetPort
		}
		if out[i].AppProtocol == nil {
			out[i].AppProtocol = l.AppProtocol
		}
	}

	return out
}

func findServicePort(live []corev1.ServicePort, want corev1.ServicePort) (corev1.ServicePort, bool) {
	for _, l := range live {
		if l.Name == want.Name && (l.Name != "" || l.Port == want.Port) {
			return l, true
		}
	}

	return corev1.ServicePort{}, false
}
