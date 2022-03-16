package machine

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				machine.ObjectMeta.Labels = map[string]string{"foo": "bar"}
				machineSet.Spec.Selector.MatchLabels = map[string]string{"no": "bar"}
				res := hasMatchingLabels(&machineSet, &machine)
				Expect(res).To(Equal(false))
			})
		})

		Context("When there are matching labels", func() {

			It("should return true", func() {
				machine.ObjectMeta.Labels = map[string]string{"foo": "bar"}
				machineSet.Spec.Selector.MatchLabels = map[string]string{"foo": "bar"}
				res := hasMatchingLabels(&machineSet, &machine)
				Expect(res).To(Equal(true))
			})
		})

	})
})
