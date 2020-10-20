package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	eirinix "code.cloudfoundry.org/eirinix"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func trivyInject(severity string) string {
	return fmt.Sprintf("curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/master/contrib/install.sh | sh -s -- -b tmp && tmp/trivy filesystem --severity '%s' --exit-code 1 --no-progress /", severity)
}

// Extension is the secscanner extension which injects a initcontainer which checks for vulnerability in the container image
type Extension struct{ Memory, Severity string }

// Handle takes a pod and inject a secscanner container if needed
func (ext *Extension) Handle(ctx context.Context, eiriniManager eirinix.Manager, pod *corev1.Pod, req admission.Request) admission.Response {

	if pod == nil {
		return admission.Errored(http.StatusBadRequest, errors.New("No pod could be decoded from the request"))
	}
	podCopy := pod.DeepCopy()

	// Stop if a secscanner was already injected
	for i := range podCopy.Spec.InitContainers {
		c := &podCopy.Spec.InitContainers[i]
		if c.Name == "secscanner" {
			return eiriniManager.PatchFromPod(req, podCopy)
		}
	}

	var image string
	for i := range podCopy.Spec.Containers {
		c := &podCopy.Spec.Containers[i]
		switch c.Name {
		case "opi":
			image = c.Image
		}
	}

	secscanner := corev1.Container{
		Name:            "secscanner",
		Image:           image,
		Args:            []string{trivyInject(ext.Severity)},
		Command:         []string{"/bin/sh", "-c"},
		ImagePullPolicy: corev1.PullAlways,
		Env:             []corev1.EnvVar{},
	}

	if len(ext.Memory) > 0 {
		q, err := resource.ParseQuantity(ext.Memory)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, errors.New("Failed parsing quantity: "+ext.Memory))
		}
		secscanner.Resources = corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceMemory: q},
			Limits:   map[corev1.ResourceName]resource.Quantity{corev1.ResourceMemory: q},
		}
	}

	podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, secscanner)

	return eiriniManager.PatchFromPod(req, podCopy)
}
