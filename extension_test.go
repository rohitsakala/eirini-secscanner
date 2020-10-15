package main_test

import (
	"context"
	"encoding/json"

	. "github.com/mudler/eirini-secscanner"

	eirinixcatalog "code.cloudfoundry.org/eirinix/testing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func jsonifyPatches(resp admission.Response) []string {
	var r []string
	for _, patch := range resp.Patches {
		r = append(r, patch.Json())
	}
	return r
}

var _ = Describe("Eirini extension", func() {
	eirinixcat := eirinixcatalog.NewCatalog()
	extension := &Extension{}
	eiriniManager := eirinixcat.SimpleManager()
	request := admission.Request{}
	pod := &corev1.Pod{}

	JustBeforeEach(func() {
		extension = &Extension{}
		eirinixcat = eirinixcatalog.NewCatalog()
		eiriniManager = eirinixcat.SimpleManager()

		raw, err := json.Marshal(pod)
		Expect(err).ToNot(HaveOccurred())

		request = admission.Request{AdmissionRequest: admissionv1beta1.AdmissionRequest{Object: runtime.RawExtension{Raw: raw}}}
	})

	Describe("eirini-dns-aliases", func() {
		Context("when handling a Eirini runtime app", func() {
			BeforeEach(func() {
				pod = &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "opi",
								Image: "foo",
							},
						},
					},
				}
			})

			It("Does patch the dns policy", func() {

				patches := jsonifyPatches(extension.Handle(context.Background(), eiriniManager, pod, request))
				container := struct {
					Command         []string    `json:"command"`
					Args            []string    `json:"args"`
					Image           string      `json:"image"`
					ImagePullPolicy string      `json:"imagePullPolicy"`
					Name            string      `json:"name"`
					Resources       interface{} `json:"resources"`
				}{
					Name:            "secscanner",
					Image:           "foo",
					ImagePullPolicy: "Always",
					Resources:       map[string]interface{}{},
					Args:            []string{TrivyInject},
					Command:         []string{"/bin/sh", "-c"},
				}
				patch := struct {
					Op    string        `json:"op"`
					Path  string        `json:"path"`
					Value []interface{} `json:"value"`
				}{
					Op:    "add",
					Path:  "/spec/initContainers",
					Value: []interface{}{container},
				}
				dataPatch, err := json.Marshal(patch)
				Expect(err).ToNot(HaveOccurred())
				Expect(patches).To(ContainElement(MatchJSON(dataPatch)))
				Expect(len(patches)).To(Equal(1))
			})
		})
	})
})
