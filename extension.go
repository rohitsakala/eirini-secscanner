package main

import (
	"context"
	"errors"
	"net/http"

	eirinix "code.cloudfoundry.org/eirinix"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const TrivyInject = `curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/master/contrib/install.sh | sh -s -- -b tmp && tmp/trivy filesystem --exit-code 1 --no-progress /`

// Extension is the secscanner extension which injects a initcontainer which checks for vulnerability in the container image
type Extension struct{}

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

	q, err := resource.ParseQuantity("1G")
	if err != nil {
		return admission.Errored(http.StatusBadRequest, errors.New("Failed parsing quantity"))
	}

	secscanner := v1.Container{
		Name:            "secscanner",
		Image:           image,
		Args:            []string{TrivyInject},
		Command:         []string{"/bin/sh", "-c"},
		ImagePullPolicy: v1.PullAlways,
		Env:             []v1.EnvVar{},
		Resources:       v1.ResourceRequirements{Requests: map[v1.ResourceName]resource.Quantity{v1.ResourceMemory: q}},
	}

	podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, secscanner)

	return eiriniManager.PatchFromPod(req, podCopy)
}
