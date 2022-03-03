package controllers

import (
	"testing"
	"time"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &ReconcileMachineSet{}

const (
	ownerControllerKind = "MachineSet"
	namespace           = "openshift-machine-api"
)

func TestShouldExcludeMachine(t *testing.T) {
	controller := true
	testCases := []struct {
		machineSet machinev1.MachineSet
		machine    machinev1.Machine
		expected   bool
	}{
		{
			machineSet: machinev1.MachineSet{},
			machine: machinev1.Machine{
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
			},
			expected: true,
		},
		{
			machineSet: machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "withMatchingLabels",
					Namespace: "test",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			expected: false,
		},
		{
			machineSet: machinev1.MachineSet{},
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "withDeletionTimestamp",
					Namespace:         "test",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		got := shouldExcludeMachine(&tc.machineSet, &tc.machine)
		if got != tc.expected {
			t.Errorf("Case %s. Got: %v, expected: %v", tc.machine.Name, got, tc.expected)
		}
	}
}

func TestHasMatchingLabels(t *testing.T) {
	testCases := []struct {
		machineSet machinev1.MachineSet
		machine    machinev1.Machine
		expected   bool
	}{
		{
			machineSet: machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "matchSelector",
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			expected: true,
		},
		{
			machineSet: machinev1.MachineSet{
				Spec: machinev1.MachineSetSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "doesNotMatchSelector",
					Labels: map[string]string{
						"no": "match",
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		got := hasMatchingLabels(&tc.machineSet, &tc.machine)
		if tc.expected != got {
			t.Errorf("Case %s. Got: %v, expected %v", tc.machine.Name, got, tc.expected)
		}
	}
}
