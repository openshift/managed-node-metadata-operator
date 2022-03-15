package int

import (
	"context"
	"fmt"
	"time"

	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Integration struct {
	Client client.Client
	mgr    manager.Manager
}

func NewIntegration() (*Integration, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(machinev1.AddToScheme(scheme))
	utilruntime.Must(admissionregistration.AddToScheme(scheme))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		return &Integration{}, err
	}
	client := mgr.GetClient()
	i := Integration{client, mgr}
	go func() {
		err := mgr.GetCache().Start(context.TODO())
		if err != nil {
			panic(err)
		}
	}()

	// Wait for cache to start. Is there a better way?
	time.Sleep(2 * time.Second)
	return &i, nil
}

func (i *Integration) Shutdown() {
}

func (i *Integration) GetWorkerMachineSet() (machinev1.MachineSet, error) {
	msList := &machinev1.MachineSetList{}
	err := i.Client.List(context.Background(), msList, client.InNamespace("openshift-machine-api"))
	if err != nil {
		return machinev1.MachineSet{}, err
	}
	for _, ms := range msList.Items {
		role, ok := ms.Labels["hive.openshift.io/machine-pool"]
		if ok && role == "worker" {
			return ms, nil
		}
	}
	return machinev1.MachineSet{}, fmt.Errorf("no worker MachineSet found")
}

func (i *Integration) GetMachinesForMachineSets(machineSet *machinev1.MachineSet) ([]*machinev1.Machine, error) {

	allMachines := &machinev1.MachineList{}

	err := i.Client.List(context.Background(), allMachines, client.InNamespace(machineSet.Namespace))
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

	if machine.ObjectMeta.DeletionTimestamp != nil {
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

func (i *Integration) GetNodeForMachine(m *machinev1.Machine) (v1.Node, error) {
	if m.Status.NodeRef == nil || m.Status.NodeRef.Name == "" {
		return v1.Node{}, nil
	}
	node := &v1.Node{}
	err := i.Client.Get(context.TODO(), types.NamespacedName{Name: m.Status.NodeRef.Name}, node)
	if err != nil {
		return v1.Node{}, err
	}
	return *node, err
}

func (i *Integration) DisableWebhook() error {
	webhook := admissionregistration.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sre-regular-user-validation",
		},
	}
	err := i.Client.Delete(context.TODO(), &webhook)
	return err
}
