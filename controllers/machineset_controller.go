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
	"fmt"
	"reflect"
	"strings"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	controllerKind = machinev1.SchemeGroupVersion.WithKind("MachineSet")
)

// Add creates a new MachineSet Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager, opts manager.Options) error {
	r := newReconciler(mgr)
	return add(mgr, r, r.MachineToMachineSets)
}

// newReconciler returns a new reconcile.Reconciler.
func newReconciler(mgr manager.Manager) *ReconcileMachineSet {
	return &ReconcileMachineSet{Client: mgr.GetClient(), scheme: mgr.GetScheme(), recorder: mgr.GetEventRecorderFor(controllerName)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler, mapFn handler.MapFunc) error {
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

func (r *ReconcileMachineSet) MachineToMachineSets(o client.Object) []reconcile.Request {
	result := []reconcile.Request{}
	m := &machinev1.Machine{}
	key := client.ObjectKey{Namespace: o.GetNamespace(), Name: o.GetName()}
	err := r.Client.Get(context.Background(), key, m)
	if err != nil {
		klog.Errorf("Unable to retrieve Machine %v from store: %v", key, err)
		return nil
	}

	for _, ref := range m.ObjectMeta.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return result
		}
	}

	mss := r.getMachineSetsForMachine(m)
	if len(mss) == 0 {
		klog.V(4).Infof("Found no machine set for machine: %v", m.Name)
		return nil
	}

	for _, ms := range mss {
		name := client.ObjectKey{Namespace: ms.Namespace, Name: ms.Name}
		result = append(result, reconcile.Request{NamespacedName: name})
	}

	return result
}

// +kubebuilder:rbac:groups=machine.openshift.io,resources=machinesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=machine.openshift.io,resources=machinesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=nodes,verbs=get;list;watch;update;patch

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

	allMachines := &machinev1.MachineList{}

	err = r.Client.List(context.Background(), allMachines, client.InNamespace(machineSet.Namespace))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list machines: %w", err)
	}

	// Make sure that label selector can match template's labels.
	// TODO(vincepri): Move to a validation (admission) webhook when supported.
	selector, err := metav1.LabelSelectorAsSelector(&machineSet.Spec.Selector)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to parse MachineSet %q label selector: %w", machineSet.Name, err)
	}

	if !selector.Matches(labels.Set(machineSet.Spec.Template.Labels)) {
		return reconcile.Result{}, fmt.Errorf("failed validation on MachineSet %q label selector, cannot match any machines ", machineSet.Name)
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

	for _, m := range machines {
		// Loop through machines and compare labels with machiset
		if !reflect.DeepEqual(machineSet.Spec.Template.Spec.Labels, m.Spec.Labels) {
			// Add labels to machine
			m.Spec.Labels = machineSet.Spec.Template.Spec.Labels
			err = r.Client.Update(ctx, m)
			if err != nil {
				klog.Errorf("failed to update label in %s", m.Name)
				return reconcile.Result{}, err
			}
		}
		// Get node for machine
		if m.Status.NodeRef == nil || m.Status.NodeRef.Name == "" {
			continue
		}
		node := &v1.Node{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: m.Status.NodeRef.Name}, node)
		if err != nil {
			klog.Errorf("failed to fetch node for machine %s", m.Name)
			return reconcile.Result{}, err
		}

		// Build temp map to store current custom labels in node
		currentNodeLabels := map[string]string{}

		currentAnnotationValue, ok := node.Annotations["managed.openshift.com/customlabels"]
		if ok {
			for _, lk := range strings.Split(currentAnnotationValue, ",") {
				if lv, nodeHasLabel := node.Labels[lk]; nodeHasLabel {
					currentNodeLabels[lk] = lv
					if _, machineHasLabel := m.Spec.Labels[lk]; !machineHasLabel {
						klog.Info("deleting label in node")
						delete(node.Labels, lk)
					}
				}
			}
		}

		// Compare custom labels with labels in machine
		if !reflect.DeepEqual(m.Spec.Labels, currentNodeLabels) {
			// Update custom labels with new machine labels added
			newAnnotationValue := ""
			for newKey, newVal := range m.Spec.Labels {
				node.Labels[newKey] = newVal
				if newAnnotationValue != "" {
					newAnnotationValue += ","
				}
				newAnnotationValue += newKey
			}
			node.Annotations["managed.openshift.com/customlabels"] = newAnnotationValue
		}

		err = r.Client.Update(ctx, node)
		if err != nil {
			klog.Errorf("failed to update label in %s", node.Name)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// shouldExcludeMachine returns true if the machine should be filtered out, false otherwise.
func shouldExcludeMachine(machineSet *machinev1.MachineSet, machine *machinev1.Machine) bool {
	// Ignore inactive machines.
	if metav1.GetControllerOf(machine) != nil && !metav1.IsControlledBy(machine, machineSet) {
		klog.V(4).Infof("%s not controlled by %v", machine.Name, machineSet.Name)
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
		klog.Warningf("unable to convert selector: %v", err)
		return false
	}

	// If a deployment with a nil or empty selector creeps in, it should match nothing, not everything.
	if selector.Empty() {
		klog.V(2).Infof("%v machineset has empty selector", machineSet.Name)
		return false
	}

	if !selector.Matches(labels.Set(machine.Labels)) {
		klog.V(4).Infof("%v machine has mismatch labels", machine.Name)
		return false
	}

	return true
}

func (c *ReconcileMachineSet) getMachineSetsForMachine(m *machinev1.Machine) []*machinev1.MachineSet {
	if len(m.Labels) == 0 {
		klog.Warningf("No machine sets found for Machine %v because it has no labels", m.Name)
		return nil
	}

	msList := &machinev1.MachineSetList{}
	err := c.Client.List(context.Background(), msList, client.InNamespace(m.Namespace))
	if err != nil {
		klog.Errorf("Failed to list machine sets, %v", err)
		return nil
	}

	var mss []*machinev1.MachineSet
	for idx := range msList.Items {
		ms := &msList.Items[idx]
		if hasMatchingLabels(ms, m) {
			mss = append(mss, ms)
		}
	}

	return mss
}
