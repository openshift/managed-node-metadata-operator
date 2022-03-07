package controllers

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	machinev1 "github.com/openshift/api/machine/v1beta1"
	v1 "k8s.io/api/core/v1"
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

var _ reconcile.Reconciler = &ReconcileMachineSet{}

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

func TestUpdateLabelsinMachine(t *testing.T) {
	testCases := []struct {
		machineSet     machinev1.MachineSet
		machine        machinev1.Machine
		updatedMachine machinev1.Machine
	}{
		{
			machineSet: machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSetSpec{
					Template: machinev1.MachineTemplateSpec{
						Spec: machinev1.MachineSpec{
							ObjectMeta: machinev1.ObjectMeta{
								Labels: map[string]string{
									"foo": "bar"},
							},
						},
					},
				},
			},
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{},
					},
				},
			},
			updatedMachine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
		},
		{
			machineSet: machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSetSpec{
					Template: machinev1.MachineTemplateSpec{
						Spec: machinev1.MachineSpec{
							ObjectMeta: machinev1.ObjectMeta{
								Labels: map[string]string{},
							},
						},
					},
				},
			},
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			updatedMachine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{},
					},
				},
			},
		},
		{
			machineSet: machinev1.MachineSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSetSpec{
					Template: machinev1.MachineTemplateSpec{
						Spec: machinev1.MachineSpec{
							ObjectMeta: machinev1.ObjectMeta{
								Labels: map[string]string{},
							},
						},
					},
				},
			},
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{},
					},
				},
			},
			updatedMachine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{},
					},
				},
			},
		},
	}

	for _, tc := range testCases {

		err := machinev1.AddToScheme(scheme.Scheme)
		if err != nil {
			fmt.Printf("failed adding to scheme")
		}
		localObjects := []runtime.Object{
			&tc.machineSet,
			&tc.machine,
		}

		mocks := &mocks{
			fakeKubeClient: fake.NewFakeClient(localObjects...),
			mockCtrl:       gomock.NewController(t),
		}
		r := ReconcileMachineSet{
			mocks.fakeKubeClient,
			scheme.Scheme,
			record.NewFakeRecorder(32),
		}

		var ctx context.Context
		err = r.updateLabelsInMachine(ctx, &tc.machineSet, &tc.machine)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(tc.machine.Spec.Labels, tc.machineSet.Spec.Template.Spec.Labels) {
			t.Errorf("Got: %v, expected %v", tc.machine.Spec.Labels, tc.updatedMachine.Spec.Labels)
		}
	}
}

func TestUpdateLabelsInNode(t *testing.T) {
	testCases := []struct {
		machine     machinev1.Machine
		node        v1.Node
		updatedNode v1.Node
	}{
		{
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Namespace:   "test",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			},
			updatedNode: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test",
					Labels: map[string]string{
						"foo": "bar",
					},
					Annotations: map[string]string{
						"managed.openshift.com/customlabels": "foo",
					},
				},
			},
		},
		{
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{},
					},
				},
			},
			node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-node",
					Namespace: "test",
					Labels: map[string]string{
						"foo": "bar",
					},
					Annotations: map[string]string{
						"managed.openshift.com/customlabels": "foo",
					},
				},
			},
			updatedNode: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Namespace:   "test",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			},
		},
		{
			machine: machinev1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test machineset",
					Namespace: "test",
				},
				Spec: machinev1.MachineSpec{
					ObjectMeta: machinev1.ObjectMeta{
						Labels: map[string]string{},
					},
				},
			},
			node: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Namespace:   "test",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			},
			updatedNode: v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-node",
					Namespace:   "test",
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			},
		},
	}

	for _, tc := range testCases {

		err := machinev1.AddToScheme(scheme.Scheme)
		if err != nil {
			fmt.Printf("failed adding to scheme")
		}
		localObjects := []runtime.Object{
			&tc.machine,
			&tc.node,
		}

		mocks := &mocks{
			fakeKubeClient: fake.NewFakeClient(localObjects...),
			mockCtrl:       gomock.NewController(t),
		}
		r := ReconcileMachineSet{
			mocks.fakeKubeClient,
			scheme.Scheme,
			record.NewFakeRecorder(32),
		}

		var ctx context.Context
		err = r.updateLabelsInNode(ctx, &tc.machine)
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(tc.machine.Spec.Labels, tc.updatedNode.Labels) {
			t.Errorf("Got: %v, expected %v", tc.node.Labels, tc.updatedNode.Labels)
		}
	}
}
