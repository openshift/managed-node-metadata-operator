package machineset

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"

	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
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

var _ = Describe("MachinesetController", func() {

	var (
		machineSet     machinev1.MachineSet
		machine        machinev1.Machine
		node           v1.Node
		updatedNode    v1.Node
		updatedMachine machinev1.Machine
		mockObjects    *mocks
		r              *ReconcileMachineSet
		ctx            context.Context
	)

	err := machinev1.AddToScheme(scheme.Scheme)
	if err != nil {
		fmt.Printf("failed adding apis to scheme in machineset controller tests")
	}

	Describe("Check if should exclude machine", func() {
		controller := true
		Context("When machine has no matching owner reference", func() {

			machineSet = machinev1.MachineSet{}
			machine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "withNoMatchingOwnerRef",
					Namespace: "test",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:       "Owner",
							Kind:       "MachineSet",
							Controller: &controller,
						},
					},
				},
			}

			It("should exclude machine", func() {
				res := shouldExcludeMachine(&machineSet, &machine)
				Expect(res).To(Equal(true))
			})
		})

		Context("When machine has matching labels", func() {
			machineSet = machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			}
			machine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "withMatchingLabels",
					Namespace: "test",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			}

			It("should not exclude machine", func() {
				res := shouldExcludeMachine(&machineSet, &machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When machine has deletion time stamp", func() {
			machineSet = machinev1.MachineSet{}
			machine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "withDeletionTimestamp",
					Namespace:         "test",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			}

			It("should exclude machine", func() {
				res := shouldExcludeMachine(&machineSet, &machine)
				Expect(res).To(Equal(true))
			})
		})

	})

	Describe("Check if machine has matching labels with machineset", func() {

		Context("When there are matching labels", func() {
			machineSet = machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
				},
			}
			machine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "matchSelector",
					Labels: map[string]string{"foo": "bar"},
				},
			}

			It("should return true", func() {
				res := hasMatchingLabels(&machineSet, &machine)
				Expect(res).To(Equal(true))
			})
		})

		Context("When there are no matching labels", func() {
			machineSet = machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
				},
			}
			machine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "matchSelector",
					Labels: map[string]string{"no": "match"},
				},
			}

			It("should return false", func() {
				res := hasMatchingLabels(&machineSet, &machine)
				Expect(res).To(Equal(false))
			})
		})

	})

	Describe("Updating labels in machine", func() {
		var (
			newLabelsInMachineSet   map[string]string
			existingLabelsInMachine map[string]string
			updatedLabelsInMachine  map[string]string
		)

		BeforeEach(func() {
			machineSet = machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSetSpec{
					Template: machinev1.MachineTemplateSpec{
						Spec: machinev1.MachineSpec{
							ObjectMeta: machinev1.ObjectMeta{
								Labels: newLabelsInMachineSet,
							},
						},
					},
				},
			}
			machine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: existingLabelsInMachine,
					},
				},
			}
			updatedMachine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: updatedLabelsInMachine,
					},
				},
			}

			localObjects := []runtime.Object{
				&machineSet,
				&machine,
			}
			mockObjects = &mocks{
				fakeKubeClient: fake.NewFakeClient(localObjects...),
				mockCtrl:       gomock.NewController(GinkgoT()),
			}

			r = &ReconcileMachineSet{
				mockObjects.fakeKubeClient,
				scheme.Scheme,
				record.NewFakeRecorder(32),
			}
		})

		AfterEach(func() {
			mockObjects.mockCtrl.Finish()
		})

		Context("When new label is added to machineset", func() {
			newLabelsInMachineSet = map[string]string{"foo": "bar"}
			existingLabelsInMachine = map[string]string{}
			updatedLabelsInMachine = map[string]string{"foo": "bar"}

			It("should update labels in machine", func() {
				err = r.updateLabelsInMachine(ctx, &machineSet, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(updatedMachine.Spec.Labels))
			})
		})

		Context("When label is deleted from machinset", func() {
			newLabelsInMachineSet = map[string]string{}
			existingLabelsInMachine = map[string]string{"foo": "bar"}
			updatedLabelsInMachine = map[string]string{}

			It("should delete label in machine", func() {
				err = r.updateLabelsInMachine(ctx, &machineSet, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(updatedMachine.Spec.Labels))
			})
		})

		Context("When no new label is added to machineset", func() {
			newLabelsInMachineSet = map[string]string{}
			existingLabelsInMachine = map[string]string{}
			updatedLabelsInMachine = map[string]string{}

			It("should not change labels", func() {
				err = r.updateLabelsInMachine(ctx, &machineSet, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(updatedMachine.Spec.Labels))
			})
		})

	})

	Describe("Updating labels in node", func() {
		var (
			newLabelsInMachine   map[string]string
			existingLabelsInNode map[string]string
			updatedLabelsInNode  map[string]string
		)

		BeforeEach(func() {
			machine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: newLabelsInMachine,
					},
				},
			}
			node = v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Namespace:   "test",
					Labels:      existingLabelsInNode,
					Annotations: existingLabelsInNode,
				},
			}

			updatedNode = v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Namespace:   "test",
					Labels:      updatedLabelsInNode,
					Annotations: updatedLabelsInNode,
				},
			}

			localObjects := []runtime.Object{
				&machine,
				&node,
			}
			mockObjects = &mocks{
				fakeKubeClient: fake.NewFakeClient(localObjects...),
				mockCtrl:       gomock.NewController(GinkgoT()),
			}

			r = &ReconcileMachineSet{
				mockObjects.fakeKubeClient,
				scheme.Scheme,
				record.NewFakeRecorder(32),
			}
		})

		AfterEach(func() {
			mockObjects.mockCtrl.Finish()
		})

		Context("When new label is added to machine", func() {
			newLabelsInMachine = map[string]string{"foo": "bar"}
			existingLabelsInNode = map[string]string{}
			updatedLabelsInNode = map[string]string{"foo": "bar"}

			It("should update labels in node", func() {
				err = r.updateLabelsInNode(ctx, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(updatedNode.Labels))
			})
		})

		Context("When label is deleted from machine", func() {
			newLabelsInMachine = map[string]string{}
			existingLabelsInNode = map[string]string{"foo": "bar"}
			updatedLabelsInNode = map[string]string{}

			It("should update labels in node", func() {
				err = r.updateLabelsInNode(ctx, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(updatedNode.Labels))
			})
		})

		Context("When no new label is added to machine", func() {
			newLabelsInMachine = map[string]string{}
			existingLabelsInNode = map[string]string{}
			updatedLabelsInNode = map[string]string{}

			It("should not change labels in node", func() {
				err = r.updateLabelsInNode(ctx, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(updatedNode.Labels))
			})
		})

	})

})
