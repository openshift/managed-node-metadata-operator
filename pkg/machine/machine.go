package machine

import (
	"context"
	"fmt"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetMachinesForMachineSet returns all machines matching the MachineSet
func GetMachinesForMachineSet(c client.Client, machineSet *machinev1.MachineSet) ([]*machinev1.Machine, error) {

	allMachines := &machinev1.MachineList{}

	err := c.List(context.Background(), allMachines, client.InNamespace(machineSet.Namespace))
	if err != nil {
		return nil, err
	}
	// Make sure that label selector can match template's labels.
	selector, err := metav1.LabelSelectorAsSelector(&machineSet.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MachineSet %q label selector: %w", machineSet.Name, err)
	}

	if !selector.Matches(labels.Set(machineSet.Spec.Template.Labels)) {
		return nil, fmt.Errorf("failed validation on MachineSet %q label selector, cannot match any machines ", machineSet.Name)
	}

	// Filter out irrelevant machines (deleting/mismatch labels)
	machines := []*machinev1.Machine{}
	for idx := range allMachines.Items {
		machine := &allMachines.Items[idx]
		if shouldExcludeMachine(machineSet, machine) {
			continue
		}

		machines = append(machines, machine)
	}

	return machines, nil
}

// shouldExcludeMachine returns true if the machine should be filtered out, false otherwise.
func shouldExcludeMachine(machineSet *machinev1.MachineSet, machine *machinev1.Machine) bool {
	// Ignore inactive machines.
	if metav1.GetControllerOf(machine) != nil && !metav1.IsControlledBy(machine, machineSet) {
		return true
	}

	if machine.DeletionTimestamp != nil {
		return true
	}

	if !hasMatchingLabels(machineSet, machine) {
		return true
	}

	return false
}

func hasMatchingLabels(machineSet *machinev1.MachineSet, machine *machinev1.Machine) bool {
	selector, err := metav1.LabelSelectorAsSelector(&machineSet.Spec.Selector)
	if err != nil {
		return false
	}

	// If a deployment with a nil or empty selector creeps in, it should match nothing, not everything.
	if selector.Empty() {
		return false
	}

	if !selector.Matches(labels.Set(machine.Labels)) {
		return false
	}

	return true
}

// GetNodeForMachine returns the node that is referenced in the machine resource
func GetNodeForMachine(c client.Client, m *machinev1.Machine) (*corev1.Node, error) {
	node := &corev1.Node{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: m.Status.NodeRef.Name}, node)
	if err != nil {
		return &corev1.Node{}, err
	}
	return node, err
}
