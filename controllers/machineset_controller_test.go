package controllers

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	m "github.com/openshift/managed-node-metadata-operator/pkg/machine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type mocks struct {
	fakeKubeClient client.Client
	mockCtrl       *gomock.Controller
}

var _ = Describe("MachinesetController", func() {
	var (
		machineSet     machinev1beta1.MachineSet
		machine        machinev1beta1.Machine
		updatedMachine machinev1beta1.Machine
		node           corev1.Node
		updatedNode    corev1.Node
		mockObjects    *mocks
		err            error
		r              *MachinesetReconciler
		ctx            context.Context
		localObjects   []client.Object
	)

	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		fmt.Printf("failed adding apis to scheme in machineset controller tests")
	}
	if err := machinev1beta1.AddToScheme(s); err != nil {
		fmt.Printf("failed adding apis to scheme in machineset controller tests")
	}

	Describe("Updating labels in machine", func() {
		var (
			newLabelsInMachineSet   map[string]string
			existingLabelsInMachine map[string]string
		)
		BeforeEach(func() {
			localObjects = []client.Object{
				&machineSet,
				&machine,
			}
		})

		JustBeforeEach(func() {
			machineSet = machinev1beta1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1beta1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"owner": "fake-machineset",
						},
					},
					Template: machinev1beta1.MachineTemplateSpec{
						ObjectMeta: machinev1beta1.ObjectMeta{
							Labels: map[string]string{
								"owner": "fake-machineset",
							},
						},
						Spec: machinev1beta1.MachineSpec{
							ObjectMeta: machinev1beta1.ObjectMeta{
								Labels: newLabelsInMachineSet,
							},
						},
					},
				},
			}
			machine = machinev1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machine",
					Namespace: "test",
					Labels: map[string]string{
						"owner": "fake-machineset",
					},
				},
				Spec: machinev1beta1.MachineSpec{
					ObjectMeta: machinev1beta1.ObjectMeta{
						Labels: existingLabelsInMachine,
					},
				},
				Status: machinev1beta1.MachineStatus{
					NodeRef: &corev1.ObjectReference{
						Name: "test-node",
					},
				},
			}

			mockObjects = &mocks{
				fakeKubeClient: fake.NewClientBuilder().WithScheme(s).WithObjects(localObjects...).Build(),
				mockCtrl:       gomock.NewController(GinkgoT()),
			}

			r = &MachinesetReconciler{
				mockObjects.fakeKubeClient,
				scheme.Scheme,
				record.NewFakeRecorder(32),
			}
		})

		AfterEach(func() {
			mockObjects.mockCtrl.Finish()
		})

		Context("When new label is added to machineset", func() {
			BeforeEach(func() {
				newLabelsInMachineSet = map[string]string{"foo": "bar"}
				existingLabelsInMachine = map[string]string{}
			})

			It("should update labels in machine", func() {
				err = r.updateLabelsInMachine(ctx, &machine, newLabelsInMachineSet)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("When label is deleted from machinset", func() {
			BeforeEach(func() {
				newLabelsInMachineSet = map[string]string{}
				existingLabelsInMachine = map[string]string{"foo": "bar"}
			})

			It("should delete label in machine", func() {
				err = r.updateLabelsInMachine(ctx, &machine, newLabelsInMachineSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(machineSet.Spec.Template.Spec.Labels))
			})
		})

		Context("When no new label is added to machineset", func() {
			BeforeEach(func() {
				newLabelsInMachineSet = map[string]string{}
				existingLabelsInMachine = map[string]string{}
			})

			It("should not change labels", func() {
				err = r.updateLabelsInMachine(ctx, &machine, newLabelsInMachineSet)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(machineSet.Spec.Template.Spec.Labels))
			})
		})

		Context("When a MachineSet would override the label of a Node", func() {
			var (
				existingLabelsInNode      map[string]string
				existingAnnotationsInNode map[string]string
			)
			BeforeEach(func() {
				newLabelsInMachineSet = map[string]string{"existingLabel": "newValue"}
				existingLabelsInMachine = map[string]string{}
				existingAnnotationsInNode = map[string]string{}
				existingLabelsInNode = map[string]string{"existingLabel": "existingValue"}
				node = corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-node",
						Labels:      existingLabelsInNode,
						Annotations: existingAnnotationsInNode,
					},
				}
				localObjects = []client.Object{
					&machine,
					&node,
				}
			})

			It("should not update the label", func() {
				result, err := r.ProcessMachineSet(context.TODO(), &machineSet)
				Expect(err).NotTo(HaveOccurred())
				newNode, _ := m.GetNodeForMachine(mockObjects.fakeKubeClient, &machine)
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(newNode.Labels).To(Equal(existingLabelsInNode))
				Expect(machine.Spec.Labels).To(Equal(existingLabelsInMachine))
			})

			Context("When the operator added it previously in the node annotation", func() {
				BeforeEach(func() {
					existingAnnotationsInNode = map[string]string{"managed.openshift.com/customlabels": "existingLabel"}
					node = corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "test-node",
							Labels:      existingLabelsInNode,
							Annotations: existingAnnotationsInNode,
						},
					}
					localObjects = []client.Object{
						&machine,
						&node,
					}
				})
				It("updates the label", func() {
					result, err := r.ProcessMachineSet(context.TODO(), &machineSet)
					Expect(err).NotTo(HaveOccurred())
					newNode, _ := m.GetNodeForMachine(mockObjects.fakeKubeClient, &machine)
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(newNode.Labels).To(Equal(newLabelsInMachineSet))
				})
			})
		})

	})

	Describe("Updating taints in machine", func() {
		var (
			newTaintsInMachineSet   []corev1.Taint
			existingTaintsInMachine []corev1.Taint
			updatedTaintsInMachine  []corev1.Taint
		)

		BeforeEach(func() {
			localObjects = []client.Object{
				&machineSet,
				&machine,
			}
		})

		JustBeforeEach(func() {
			machineSet = machinev1beta1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1beta1.MachineSetSpec{
					Template: machinev1beta1.MachineTemplateSpec{
						Spec: machinev1beta1.MachineSpec{
							Taints: newTaintsInMachineSet,
						},
					},
				},
			}
			machine = machinev1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1beta1.MachineSpec{
					Taints: existingTaintsInMachine,
				},
			}
			updatedMachine = machinev1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1beta1.MachineSpec{
					Taints: updatedTaintsInMachine,
				},
			}

			mockObjects = &mocks{
				fakeKubeClient: fake.NewClientBuilder().WithScheme(s).WithObjects(localObjects...).Build(),
				mockCtrl:       gomock.NewController(GinkgoT()),
			}

			r = &MachinesetReconciler{
				mockObjects.fakeKubeClient,
				scheme.Scheme,
				record.NewFakeRecorder(32),
			}
		})

		AfterEach(func() {
			mockObjects.mockCtrl.Finish()
		})

		Context("When new taint is added to machineset", func() {

			BeforeEach(func() {
				newTaintsInMachineSet = []corev1.Taint{
					{
						Effect: corev1.TaintEffectPreferNoSchedule,
						Value:  "bar",
						Key:    "foo",
					},
				}
				existingTaintsInMachine = []corev1.Taint{}
				updatedTaintsInMachine = []corev1.Taint{
					{
						Effect: corev1.TaintEffectPreferNoSchedule,
						Value:  "bar",
						Key:    "foo",
					},
				}
			})

			It("should update taints in machine", func() {
				err = r.updateTaintsInMachine(ctx, &machineSet, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Taints).To(Equal(updatedMachine.Spec.Taints))
			})
		})

		Context("When taint is deleted from machinset", func() {

			BeforeEach(func() {
				newTaintsInMachineSet = []corev1.Taint{}
				existingTaintsInMachine = []corev1.Taint{
					{
						Effect: corev1.TaintEffectPreferNoSchedule,
						Value:  "bar",
						Key:    "foo",
					}}
				updatedTaintsInMachine = []corev1.Taint{}
			})

			It("should delete taint in machine", func() {
				err = r.updateTaintsInMachine(ctx, &machineSet, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Taints).To(Equal(updatedMachine.Spec.Taints))
			})
		})

		Context("When no new taint is added to machineset", func() {

			BeforeEach(func() {
				newTaintsInMachineSet = []corev1.Taint{}
				existingTaintsInMachine = []corev1.Taint{}
				updatedTaintsInMachine = []corev1.Taint{}
			})

			It("should not change taints", func() {
				err = r.updateTaintsInMachine(ctx, &machineSet, &machine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Taints).To(Equal(updatedMachine.Spec.Taints))
			})
		})

	})

	Describe("Updating labels in node", func() {
		var (
			newLabelsInMachine        map[string]string
			existingLabelsInNode      map[string]string
			existingAnnotationsInNode map[string]string
			node                      corev1.Node
		)

		JustBeforeEach(func() {
			machine = machinev1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machine",
					Namespace: "test",
				},
				Spec: machinev1beta1.MachineSpec{
					ObjectMeta: machinev1beta1.ObjectMeta{
						Labels: newLabelsInMachine,
					},
				},
			}
			node = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Labels:      existingLabelsInNode,
					Annotations: existingAnnotationsInNode,
				},
			}

			localObjects := []client.Object{
				&machine,
				&node,
			}
			mockObjects = &mocks{
				fakeKubeClient: fake.NewClientBuilder().WithScheme(s).WithObjects(localObjects...).Build(),
				mockCtrl:       gomock.NewController(GinkgoT()),
			}

			r = &MachinesetReconciler{
				mockObjects.fakeKubeClient,
				scheme.Scheme,
				record.NewFakeRecorder(32),
			}
		})

		AfterEach(func() {
			mockObjects.mockCtrl.Finish()
		})

		Context("When new label is added to machine", func() {
			BeforeEach(func() {
				newLabelsInMachine = map[string]string{"foo": "bar"}
				existingLabelsInNode = map[string]string{}
			})

			It("should update labels in node", func() {
				err = r.updateLabelsInNode(ctx, &node, newLabelsInMachine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(node.Labels))
			})
		})

		Context("When label is deleted from machine", func() {
			BeforeEach(func() {
				newLabelsInMachine = map[string]string{}
				existingLabelsInNode = map[string]string{"foo": "bar"}
				existingAnnotationsInNode = map[string]string{"managed.openshift.com/customlabels": "foo"}
			})

			It("should update labels in node", func() {
				var myNilMap map[string]string
				err = r.updateLabelsInNode(ctx, &node, newLabelsInMachine)
				Expect(err).NotTo(HaveOccurred())
				Expect(node.Labels).To(Equal(myNilMap))
			})
		})

		Context("When no new label is added to machine", func() {
			BeforeEach(func() {
				newLabelsInMachine = map[string]string{}
				existingLabelsInNode = map[string]string{}
			})

			It("should not change labels in node", func() {
				err = r.updateLabelsInNode(ctx, &node, newLabelsInMachine)
				Expect(err).NotTo(HaveOccurred())
				Expect(machine.Spec.Labels).To(Equal(node.Labels))
			})
		})

	})

	Describe("Updating taints in node", func() {
		var (
			newTaintsInMachine   []corev1.Taint
			existingTaintsInNode []corev1.Taint
			updatedTaintsInNode  []corev1.Taint
		)

		BeforeEach(func() {
			localObjects = []client.Object{
				&machine,
				&node,
			}
		})
		JustBeforeEach(func() {
			machine = machinev1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machine",
					Namespace: "test",
				},
				Spec: machinev1beta1.MachineSpec{
					Taints: newTaintsInMachine,
				},
			}
			node = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test",
				},
				Spec: corev1.NodeSpec{
					Taints: existingTaintsInNode,
				},
			}

			updatedNode = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test",
				},
				Spec: corev1.NodeSpec{
					Taints: updatedTaintsInNode,
				},
			}

			mockObjects = &mocks{
				fakeKubeClient: fake.NewClientBuilder().WithScheme(s).WithObjects(localObjects...).Build(),
				mockCtrl:       gomock.NewController(GinkgoT()),
			}

			r = &MachinesetReconciler{
				mockObjects.fakeKubeClient,
				scheme.Scheme,
				record.NewFakeRecorder(32),
			}
		})

		AfterEach(func() {
			mockObjects.mockCtrl.Finish()
		})

		Context("When new taint is added to machine", func() {

			BeforeEach(func() {
				newTaintsInMachine = []corev1.Taint{
					{
						Effect: corev1.TaintEffectPreferNoSchedule,
						Value:  "bar",
						Key:    "foo",
					},
				}
				existingTaintsInNode = []corev1.Taint{}
				updatedTaintsInNode = []corev1.Taint{
					{
						Effect: corev1.TaintEffectPreferNoSchedule,
						Value:  "bar",
						Key:    "foo",
					},
				}
			})

			It("should update taints in node", func() {
				err = r.updateTaintsInNode(ctx, &machine, &node)
				Expect(err).NotTo(HaveOccurred())
				Expect(node.Spec.Taints).To(Equal(updatedNode.Spec.Taints))
			})
		})

		Context("When a NoSchedule taint is added to the node", func() {

			BeforeEach(func() {
				newTaintsInMachine = []corev1.Taint{}
				existingTaintsInNode = []corev1.Taint{
					{
						Key:    corev1.TaintNodeUnschedulable,
						Effect: corev1.TaintEffectNoSchedule,
					},
				}
				updatedTaintsInNode = []corev1.Taint{
					{
						Key:    corev1.TaintNodeUnschedulable,
						Effect: corev1.TaintEffectNoSchedule,
					},
				}
			})

			It("should update taints in node", func() {
				err = r.updateTaintsInNode(ctx, &machine, &node)
				Expect(err).NotTo(HaveOccurred())
				Expect(node.Spec.Taints).To(Equal(updatedNode.Spec.Taints))
			})
		})

		Context("When taint is deleted from machinset", func() {

			BeforeEach(func() {
				newTaintsInMachine = []corev1.Taint{}
				existingTaintsInNode = []corev1.Taint{
					{
						Effect: corev1.TaintEffectPreferNoSchedule,
						Value:  "bar",
						Key:    "foo",
					}}
				updatedTaintsInNode = []corev1.Taint{}
			})

			It("should delete taint in node", func() {
				err = r.updateTaintsInNode(ctx, &machine, &node)
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedNode.Spec.Taints).To(Equal(updatedTaintsInNode))
			})
		})

		Context("When no new taint is added to machine", func() {

			BeforeEach(func() {
				newTaintsInMachine = []corev1.Taint{}
				existingTaintsInNode = []corev1.Taint{}
				updatedTaintsInNode = []corev1.Taint{}

			})

			It("should not change taints", func() {
				err = r.updateTaintsInNode(ctx, &machine, &node)
				Expect(err).NotTo(HaveOccurred())
				Expect(node.Spec.Taints).To(Equal(updatedNode.Spec.Taints))
			})
		})

		Context("When a duplicate taint is added", func() {
			BeforeEach(func() {
				newTaintsInMachine = []corev1.Taint{
					corev1.Taint{
						Key:    "test",
						Value:  "test",
						Effect: "NoSchedule",
					},
					corev1.Taint{
						Key:    "test",
						Value:  "test",
						Effect: "NoSchedule",
					},
				}
				existingTaintsInNode = []corev1.Taint{}
				updatedTaintsInNode = []corev1.Taint{
					corev1.Taint{
						Key:    "test",
						Value:  "test",
						Effect: "NoSchedule",
					},
				}
			})
			It("it should update the node, but indicate the error", func() {
				err = r.updateTaintsInNode(ctx, &machine, &node)
				Expect(err).To(HaveOccurred())
				Expect(node.Spec.Taints).To(Equal(updatedNode.Spec.Taints))
			})
		})
	})
})
