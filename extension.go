package secscanner

import (
	"context"
	"errors"
	"net/http"

	eirinix "code.cloudfoundry.org/eirinix"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Extension is the secscanner extension which injects a initcontainer which checks for vulnerability in the container image
type Extension struct{}

func (ext *Extension) Handle(ctx context.Context, eiriniManager eirinix.Manager, pod *corev1.Pod, req admission.Request) admission.Response {

	if pod == nil {
		return admission.Errored(http.StatusBadRequest, errors.New("No pod could be decoded from the request"))
	}

	//_, file, _, _ := runtime.Caller(0)
	//	log := eiriniManager.GetLogger().Named(file)

	podCopy := pod.DeepCopy()

	for i := range podCopy.Spec.InitContainers {
		c := &podCopy.Spec.InitContainers[i]
		if c.Name == "secscanner" {
			return eiriniManager.PatchFromPod(req, podCopy)
		}
	}

	secscanner := v1.Container{
		Name:            "secscanner",
		Image:           "alpine",
		Args:            []string{"loggregator"},
		Command:         []string{},
		ImagePullPolicy: v1.PullAlways,
		Env:             []v1.EnvVar{},
	}

	podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, secscanner)

	return eiriniManager.PatchFromPod(req, podCopy)
}
