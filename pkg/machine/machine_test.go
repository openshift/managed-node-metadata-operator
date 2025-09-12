package machine

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Machine", func() {

	var (
		machineSet machinev1.MachineSet
		machine    machinev1.Machine
	)

	Describe("Check if should exclude machine", func() {
		controller := true
		Context("When machine has no matching owner reference", func() {

			It("should exclude machine", func() {

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

				res := shouldExcludeMachine(&machineSet, &machine)
				Expect(res).To(Equal(true))
			})
		})

		Context("When machine has matching labels", func() {

			It("should not exclude machine", func() {

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

				res := shouldExcludeMachine(&machineSet, &machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When machine has deletion time stamp", func() {

			It("should exclude machine", func() {
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

				res := shouldExcludeMachine(&machineSet, &machine)
				Expect(res).To(Equal(true))
			})
		})

	})

	Describe("Check if machine has matching labels with machineset", func() {

		BeforeEach(func() {
			machineSet = machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{},
					},
				},
			}
			machine = machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "matchSelector",
					Labels: map[string]string{},
				},
			}
		})

		Context("When there are no matching labels", func() {

			It("should return false", func() {
				machine.Labels = map[string]string{"foo": "bar"}
				machineSet.Spec.Selector.MatchLabels = map[string]string{"no": "bar"}
				res := hasMatchingLabels(&machineSet, &machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When there are matching labels", func() {

			It("should return true", func() {
				machine.Labels = map[string]string{"foo": "bar"}
				machineSet.Spec.Selector.MatchLabels = map[string]string{"foo": "bar"}
				res := hasMatchingLabels(&machineSet, &machine)
				Expect(res).To(Equal(true))
			})
		})

	})

	Describe("GetMachinesForMachineSet", func() {
		var (
			fakeClient client.Client
			machineSet *machinev1.MachineSet
		)

		BeforeEach(func() {
			scheme := runtime.NewScheme()
			_ = machinev1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

			machineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machineset",
					Namespace: "test-namespace",
				},
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"machine.openshift.io/cluster-api-machineset": "test-machineset",
						},
					},
					Template: machinev1.MachineTemplateSpec{
						ObjectMeta: machinev1.ObjectMeta{
							Labels: map[string]string{
								"machine.openshift.io/cluster-api-machineset": "test-machineset",
							},
						},
					},
				},
			}
		})

		Context("When no machines exist", func() {
			It("should return empty list", func() {
				machines, err := GetMachinesForMachineSet(fakeClient, machineSet)
				Expect(err).To(BeNil())
				Expect(machines).To(HaveLen(0))
			})
		})

		Context("When machines exist with matching labels", func() {
			BeforeEach(func() {
				machine1 := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"machine.openshift.io/cluster-api-machineset": "test-machineset",
						},
					},
				}
				machine2 := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-2",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"machine.openshift.io/cluster-api-machineset": "test-machineset",
						},
					},
				}
				err := fakeClient.Create(context.Background(), machine1)
				Expect(err).To(BeNil())
				err = fakeClient.Create(context.Background(), machine2)
				Expect(err).To(BeNil())
			})

			It("should return matching machines", func() {
				machines, err := GetMachinesForMachineSet(fakeClient, machineSet)
				Expect(err).To(BeNil())
				Expect(machines).To(HaveLen(2))
			})
		})

		Context("When machines exist with non-matching labels", func() {
			BeforeEach(func() {
				machine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"machine.openshift.io/cluster-api-machineset": "other-machineset",
						},
					},
				}
				err := fakeClient.Create(context.Background(), machine)
				Expect(err).To(BeNil())
			})

			It("should return empty list", func() {
				machines, err := GetMachinesForMachineSet(fakeClient, machineSet)
				Expect(err).To(BeNil())
				Expect(machines).To(HaveLen(0))
			})
		})

		Context("When machines exist in different namespace", func() {
			BeforeEach(func() {
				machine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "machine-1",
						Namespace: "different-namespace",
						Labels: map[string]string{
							"machine.openshift.io/cluster-api-machineset": "test-machineset",
						},
					},
				}
				err := fakeClient.Create(context.Background(), machine)
				Expect(err).To(BeNil())
			})

			It("should not return machines from different namespace", func() {
				machines, err := GetMachinesForMachineSet(fakeClient, machineSet)
				Expect(err).To(BeNil())
				Expect(machines).To(HaveLen(0))
			})
		})

		Context("When label selector doesn't match template labels", func() {
			BeforeEach(func() {
				machineSet.Spec.Selector.MatchLabels = map[string]string{
					"machine.openshift.io/cluster-api-machineset": "different-machineset",
				}
			})

			It("should return error", func() {
				machines, err := GetMachinesForMachineSet(fakeClient, machineSet)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed validation on MachineSet"))
				Expect(machines).To(BeNil())
			})
		})

		Context("When label selector is invalid", func() {
			BeforeEach(func() {
				machineSet.Spec.Selector = metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "invalid",
							Operator: "InvalidOperator",
						},
					},
				}
			})

			It("should return error", func() {
				machines, err := GetMachinesForMachineSet(fakeClient, machineSet)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to parse MachineSet"))
				Expect(machines).To(BeNil())
			})
		})
	})

	Describe("GetNodeForMachine", func() {
		var (
			fakeClient client.Client
			machine    *machinev1.Machine
			node       *corev1.Node
		)

		BeforeEach(func() {
			scheme := runtime.NewScheme()
			_ = machinev1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)
			fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			}
			machine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: "test-namespace",
				},
				Status: machinev1.MachineStatus{
					NodeRef: &corev1.ObjectReference{
						Name: "test-node",
					},
				},
			}
		})

		Context("When node exists", func() {
			BeforeEach(func() {
				err := fakeClient.Create(context.Background(), node)
				Expect(err).To(BeNil())
			})

			It("should return the node", func() {
				resultNode, err := GetNodeForMachine(fakeClient, machine)
				Expect(err).To(BeNil())
				Expect(resultNode.Name).To(Equal("test-node"))
			})
		})

		Context("When node doesn't exist", func() {
			It("should return error", func() {
				resultNode, err := GetNodeForMachine(fakeClient, machine)
				Expect(err).ToNot(BeNil())
				Expect(resultNode).ToNot(BeNil())
			})
		})

		Context("When machine has empty node reference name", func() {
			BeforeEach(func() {
				machine.Status.NodeRef = &corev1.ObjectReference{
					Name: "",
				}
			})

			It("should return error", func() {
				resultNode, err := GetNodeForMachine(fakeClient, machine)
				Expect(err).ToNot(BeNil())
				Expect(resultNode).ToNot(BeNil())
			})
		})
	})

	Describe("hasMatchingLabels edge cases", func() {
		var (
			machineSet *machinev1.MachineSet
			machine    *machinev1.Machine
		)

		BeforeEach(func() {
			machineSet = &machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			}
			machine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-machine",
					Labels: map[string]string{},
				},
			}
		})

		Context("When selector is empty", func() {
			BeforeEach(func() {
				machineSet.Spec.Selector = metav1.LabelSelector{}
			})

			It("should return false", func() {
				res := hasMatchingLabels(machineSet, machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When machine has nil labels", func() {
			BeforeEach(func() {
				machine.Labels = nil
			})

			It("should return false", func() {
				res := hasMatchingLabels(machineSet, machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When selector has invalid expression", func() {
			BeforeEach(func() {
				machineSet.Spec.Selector = metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "foo",
							Operator: "InvalidOperator",
						},
					},
				}
			})

			It("should return false", func() {
				res := hasMatchingLabels(machineSet, machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When selector has valid expression but no match", func() {
			BeforeEach(func() {
				machineSet.Spec.Selector = metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "foo",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"bar"},
						},
					},
				}
				machine.Labels = map[string]string{"foo": "baz"}
			})

			It("should return false", func() {
				res := hasMatchingLabels(machineSet, machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When selector has valid expression with match", func() {
			BeforeEach(func() {
				machineSet.Spec.Selector = metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "foo",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"bar"},
						},
					},
				}
				machine.Labels = map[string]string{"foo": "bar"}
			})

			It("should return true", func() {
				res := hasMatchingLabels(machineSet, machine)
				Expect(res).To(Equal(true))
			})
		})
	})

	Describe("shouldExcludeMachine edge cases", func() {
		var (
			machineSet *machinev1.MachineSet
			machine    *machinev1.Machine
		)

		BeforeEach(func() {
			machineSet = &machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-machineset",
					UID:  "test-uid",
				},
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			}
			machine = &machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-machine",
					Labels: map[string]string{"foo": "bar"},
				},
			}
		})

		Context("When machine has no owner reference", func() {
			It("should not exclude machine", func() {
				res := shouldExcludeMachine(machineSet, machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When machine has matching owner reference", func() {
			BeforeEach(func() {
				controller := true
				machine.OwnerReferences = []metav1.OwnerReference{
					{
						Name:       "test-machineset",
						Kind:       "MachineSet",
						Controller: &controller,
						UID:        "test-uid",
					},
				}
			})

			It("should not exclude machine", func() {
				res := shouldExcludeMachine(machineSet, machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When machine has non-matching labels", func() {
			BeforeEach(func() {
				machine.Labels = map[string]string{"foo": "baz"}
			})

			It("should exclude machine", func() {
				res := shouldExcludeMachine(machineSet, machine)
				Expect(res).To(Equal(true))
			})
		})

		Context("When machine has no labels", func() {
			BeforeEach(func() {
				machine.Labels = nil
			})

			It("should exclude machine", func() {
				res := shouldExcludeMachine(machineSet, machine)
				Expect(res).To(Equal(true))
			})
		})
	})

	// Test the filtering logic directly
	Describe("Machine filtering logic", func() {
		Context("When testing shouldExcludeMachine with deletion timestamp", func() {
			It("should exclude machines with deletion timestamp", func() {
				machineSet := &machinev1.MachineSet{
					Spec: machinev1.MachineSetSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
					},
				}
				machine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "machine-1",
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
						Labels:            map[string]string{"foo": "bar"},
					},
				}

				res := shouldExcludeMachine(machineSet, machine)
				Expect(res).To(Equal(true))
			})
		})

		Context("When testing shouldExcludeMachine with different owner references", func() {
			It("should exclude machines with different owner references", func() {
				controller := true
				machineSet := &machinev1.MachineSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-machineset",
						UID:  "test-uid",
					},
					Spec: machinev1.MachineSetSpec{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
					},
				}
				machine := &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "machine-1",
						Labels: map[string]string{"foo": "bar"},
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "other-machineset",
								Kind:       "MachineSet",
								Controller: &controller,
								UID:        "other-uid",
							},
						},
					},
				}

				res := shouldExcludeMachine(machineSet, machine)
				Expect(res).To(Equal(true))
			})
		})
	})
})
