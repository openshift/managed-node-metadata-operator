package controllers

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"

	//. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	machinev1 "github.com/openshift/api/machine/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type mocks struct {
	fakeKubeClient client.Client
	mockCtrl       *gomock.Controller
}

var _ = Describe("MachineSet Reconciler", func() {

	var machine machinev1.Machine
	var node v1.Node
	var updatedNode v1.Node
	var mockObjects *mocks
	var r ReconcileMachineSet

	err := machinev1.AddToScheme(scheme.Scheme)
	if err != nil {
		fmt.Printf("failed adding apis to scheme in machineset controller tests")
	}

	BeforeEach(func() {
		localObjects := []runtime.Object{
			&machine,
			&node,
		}
		mockObjects = &mocks{
			fakeKubeClient: fake.NewFakeClient(localObjects...),
			mockCtrl:       gomock.NewController(GinkgoT()),
		}

		r = ReconcileMachineSet{
			mockObjects.fakeKubeClient,
			scheme.Scheme,
			record.NewFakeRecorder(32),
		}
	})

	It("hello world test", func() {
		machine = machinev1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test machineset",
				Namespace: "test",
			},
			Spec: machinev1.MachineSpec{
				ObjectMeta: machinev1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
		}
		node = v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-node",
				Namespace:   "test",
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		}

		updatedNode = v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-node",
				Namespace:   "test",
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
		}

		var ctx context.Context
		err = r.updateLabelsInNode(ctx, &machine)
		Expect(err).NotTo(HaveOccurred())
		Expect(machine.Spec.Labels).To(Equal(updatedNode.Labels))
	})

})
