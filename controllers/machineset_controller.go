/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"reflect"
	"strings"

	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	m "github.com/openshift/managed-node-metadata-operator/pkg/machine"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	controllerName = "machineset_controller"
)

// Add creates a new MachineSet Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager, opts manager.Options) error {
	r := newReconciler(mgr)
	return add(mgr, r)
}

// newReconciler returns a new reconcile.Reconciler.
func newReconciler(mgr manager.Manager) *ReconcileMachineSet {
	return &ReconcileMachineSet{Client: mgr.GetClient(), scheme: mgr.GetScheme(), recorder: mgr.GetEventRecorderFor(controllerName)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to MachineSet.
	err = c.Watch(
		&source.Kind{Type: &machinev1.MachineSet{}},
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return err
	}

	return nil

}

// ReconcileMachineSet reconciles a MachineSet object
type ReconcileMachineSet struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=machine.openshift.io,resources=machinesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=machine.openshift.io,resources=machines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=machine.openshift.io,resources=machinesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

func (r *ReconcileMachineSet) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	// Fetch the MachineSet instance
	machineSet := &machinev1.MachineSet{}
	err := r.Client.Get(ctx, request.NamespacedName, machineSet)
	if err != nil {
		if k8serr.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	return r.ProcessMachineSet(ctx, machineSet)
}

func (r *ReconcileMachineSet) ProcessMachineSet(ctx context.Context, machineSet *machinev1.MachineSet) (reconcile.Result, error) {
	// Get machines for machineset
	machines, err := m.GetMachinesForMachineSet(r, machineSet)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, machine := range machines {
		if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
			continue
		}
		node, err := m.GetNodeForMachine(r, machine)
		if err != nil {
			klog.Errorf("failed to fetch node for machine %s", machine.Name)
			return reconcile.Result{}, err
		}
		expectedLabels := r.getExpectedLabels(ctx, machineSet, machine, node)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Update labels in machine
		err = r.updateLabelsInMachine(ctx, machine, expectedLabels)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Update taints in machine
		err = r.updateTaintsInMachine(ctx, machineSet, machine)
		if err != nil {
			return reconcile.Result{}, err
		}
		//Update labels in node
		err = r.updateLabelsInNode(ctx, node, expectedLabels)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Update taints in node
		err = r.updateTaintsInNode(ctx, machine, node)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileMachineSet) getExpectedLabels(ctx context.Context, machineSet *machinev1.MachineSet, machine *machinev1.Machine, node *v1.Node) map[string]string {
	result := machineSet.Spec.Template.Spec.Labels

	currentAnnotationValue := node.Annotations["managed.openshift.com/customlabels"]
	// Labels that are already set at the Node, but weren't set by the machine resource are ignored to avoid overwriting them
	for label, _ := range machineSet.Spec.Template.Spec.Labels {
		_, nodeHasLabel := node.Labels[label]
		_, machineHasLabel := machine.Spec.Labels[label]

		// If the label exists in the annotation, it was previously set by MNMO, so it can be updated
		isSetByManageNodeMetadataOperator := false
		for _, lk := range strings.Split(currentAnnotationValue, ",") {
			if lk == label {
				isSetByManageNodeMetadataOperator = true
			}
		}
		if nodeHasLabel && !machineHasLabel && !isSetByManageNodeMetadataOperator {
			delete(result, label)
		}
	}

	return result
}

func (r *ReconcileMachineSet) updateLabelsInMachine(ctx context.Context, m *machinev1.Machine, expectedLabels map[string]string) error {
	if reflect.DeepEqual(expectedLabels, m.Spec.Labels) {
		return nil
	}
	m.Spec.Labels = expectedLabels
	err := r.Client.Update(ctx, m)
	if err != nil {
		klog.Errorf("failed to update label in %s", m.Name)
		return err
	}
	return nil
}

func (r ReconcileMachineSet) updateTaintsInMachine(ctx context.Context, machineSet *machinev1.MachineSet, m *machinev1.Machine) error {
	// Compare labels of machineset vs machine and update them if they're not the same
	if !reflect.DeepEqual(machineSet.Spec.Template.Spec.Taints, m.Spec.Taints) {
		m.Spec.Taints = machineSet.Spec.Template.Spec.Taints
	}

	err := r.Client.Update(ctx, m)
	if err != nil {
		klog.Errorf("failed to update taint in %s", m.Name)
		return err
	}
	return nil
}

func (r *ReconcileMachineSet) updateLabelsInNode(ctx context.Context, node *v1.Node, expectedLabels map[string]string) error {
	// Build temp map to store current custom labels in node
	currentNodeLabels := map[string]string{}
	// Check node Annotations and compare with Labels to get custom labels
	currentAnnotationValue, ok := node.Annotations["managed.openshift.com/customlabels"]
	if ok {
		for _, lk := range strings.Split(currentAnnotationValue, ",") {
			if lv, nodeHasLabel := node.Labels[lk]; nodeHasLabel {
				currentNodeLabels[lk] = lv
				// Delete label if it's present in node but not in machine
				if _, machineHasLabel := expectedLabels[lk]; !machineHasLabel {
					delete(node.Labels, lk)
				}
			}
		}
	}

	// Compare custom labels with labels in machine
	if reflect.DeepEqual(expectedLabels, currentNodeLabels) {
		return nil
	}
	// Update Annotations and Labels when new labels are added in machine
	newAnnotationValue := ""
	for newKey, newVal := range expectedLabels {
		node.ObjectMeta.Labels[newKey] = newVal
		if newAnnotationValue != "" {
			newAnnotationValue += ","
		}
		newAnnotationValue += newKey
	}
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	node.ObjectMeta.Annotations["managed.openshift.com/customlabels"] = newAnnotationValue

	err := r.Client.Update(ctx, node)
	if err != nil {
		klog.Errorf("failed to update label in %s", node.Name)
		return err
	}

	return nil
}

func (r ReconcileMachineSet) updateTaintsInNode(ctx context.Context, machine *machinev1.Machine, node *v1.Node) error {

	// Compare labels of machineset vs machine and update them if they're not the same
	if !reflect.DeepEqual(machine.Spec.Taints, node.Spec.Taints) {
		node.Spec.Taints = machine.Spec.Taints
	}

	err := r.Client.Update(ctx, node)
	if err != nil {
		klog.Errorf("failed to update taint in %s", node.Name)
		return err
	}
	return nil
}
