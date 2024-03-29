package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/snorwin/k8s-generic-webhook/pkg/webhook"
)

type Webhook struct {
	webhook.MutatingWebhook
	client.Client
	clientSet *kubernetes.Clientset
	Scheme    *runtime.Scheme

	SidecarImage, LogLevel string
}

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:webhook:verbs=create;update,path=/mutate-entangle-kairos-x-io-v1alpha1-entanglement,mutating=true,failurePolicy=fail,sideEffects=None,groups=core,resources=pods,versions=v1,name=mentanglement.kb.io,admissionReviewVersions={v1,v1alpha1}

var (
	EntanglementNameLabel      = "entanglement.kairos.io/name"
	EntanglementServiceLabel   = "entanglement.kairos.io/service"
	EntanglementDirectionLabel = "entanglement.kairos.io/direction"
	EntanglementNetHost        = "entanglement.kairos.io/nethost"
	EntanglementPortLabel      = "entanglement.kairos.io/target_port"
	EntanglementHostLabel      = "entanglement.kairos.io/host"
	EnvPrefix                  = "entanglement.kairos.io/env."
)

func (w *Webhook) SetupWebhookWithManager(mgr manager.Manager) error {
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	w.clientSet = clientset

	return webhook.NewGenericWebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		WithMutatePath("/mutate-entangle-kairos-x-io-v1alpha1-entanglement").
		Complete(w)
}

func (w *Webhook) Mutate(ctx context.Context, request admission.Request, object runtime.Object) admission.Response {
	_ = log.FromContext(ctx)

	pod := object.(*corev1.Pod)

	// Let user use both label and annotations
	info := make(map[string]string)

	// Annotations take precedence
	for k, v := range pod.Labels {
		info[k] = v
	}

	// Annotations take precedence
	for k, v := range pod.Annotations {
		info[k] = v
	}

	entanglementName, exists := info[EntanglementNameLabel]
	if !exists {
		return admission.Allowed("")
	}

	envs := []corev1.EnvVar{
		{
			Name: "EDGEVPNTOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "network_token",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: entanglementName,
					},
				},
			},
		}}

	for k, v := range info {
		if strings.HasPrefix(k, EnvPrefix) {
			env := strings.ReplaceAll(k, EnvPrefix, "")
			envs = append(envs, corev1.EnvVar{Name: env, Value: v})
		}
	}

	entanglementPort, exists := info[EntanglementPortLabel]
	if !exists {
		return admission.Allowed("")
	}

	cmd := "service-connect"
	entanglementDirection, exists := info[EntanglementDirectionLabel]
	if exists && entanglementDirection == "entangle" {
		cmd = "service-add"
	}

	host := "127.0.0.1"
	entanglementHost, exists := info[EntanglementHostLabel]
	if exists && entanglementHost != "" {
		host = entanglementHost
	}

	entanglementService, exists := info[EntanglementServiceLabel]
	if !exists {
		return admission.Allowed("")
	}

	podCopy := pod.DeepCopy()

	hostNetwork, exists := info[EntanglementNetHost]
	// By default it injects hostnetwork, however if set to false it does enforces it to false
	if exists && hostNetwork == "false" {
		podCopy.Spec.HostNetwork = false
	} else {
		podCopy.Spec.HostNetwork = true
	}

	secret, err := w.clientSet.CoreV1().Secrets(request.Namespace).Get(context.Background(), entanglementName, v1.GetOptions{})
	if err != nil || secret == nil {
		return admission.Denied("entanglement secret not found:  " + entanglementName + err.Error())
	}

	privileged := false

	for _, p := range podCopy.Spec.Containers {
		if p.Name == "entanglement" {
			return admission.Allowed("already entangled")
		}
	}

	servingContainer := corev1.Container{
		ImagePullPolicy: corev1.PullAlways,
		Command:         []string{"/usr/bin/edgevpn"},
		Args:            []string{cmd, entanglementService, fmt.Sprintf("%s:%s", host, entanglementPort), "--log-level", w.LogLevel},
		Env:             envs,
		SecurityContext: &corev1.SecurityContext{Privileged: &privileged},
		Name:            "entanglement",
		Image:           w.SidecarImage,
	}
	podCopy.Spec.Containers = append(podCopy.Spec.Containers, servingContainer)

	return patchFromPod(request, podCopy)
}

func patchFromPod(req admission.Request, pod *corev1.Pod) admission.Response {
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}
