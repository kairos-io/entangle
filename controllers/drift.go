package controllers

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

// podSpecNeedsUpdate reports whether the live pod template still carries the
// configuration the custom resource asks for.
//
// Only the fields the generators actually fill from the spec are compared. The
// rest of the pod template is left alone on purpose: the API server defaults a
// good deal of it (a HostPath volume gets a Type, a container gets a
// TerminationMessagePath), and comparing a defaulted field against the value we
// built would make every reconcile issue an update.
func podSpecNeedsUpdate(desired, live v1.PodSpec) bool {
	if desired.HostNetwork != live.HostNetwork {
		return true
	}

	if len(desired.Containers) != len(live.Containers) {
		return true
	}

	for i, want := range desired.Containers {
		got := live.Containers[i]
		if want.Name != got.Name ||
			want.Image != got.Image ||
			!equality.Semantic.DeepEqual(want.Command, got.Command) ||
			!equality.Semantic.DeepEqual(want.Args, got.Args) ||
			!equality.Semantic.DeepEqual(want.Env, got.Env) {
			return true
		}
	}

	return false
}
