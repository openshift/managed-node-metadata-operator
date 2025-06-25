/*
Copyright 2022.

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

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	m "github.com/openshift/managed-node-metadata-operator/pkg/machine"
	"github.com/openshift/managed-node-metadata-operator/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MachinesetReconciler reconciles a Machineset object
type MachinesetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

type DuplicateTaintError struct {
	Message string
}

func (d DuplicateTaintError) Error() string {
	return d.Message
}

//+kubebuilder:rbac:groups=machine.openshift.io,resources=machinesets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=machine.openshift.io,resources=machinesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=machine.openshift.io,resources=machines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Machineset object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *MachinesetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch the MachineSet instance
	machineSet := &machinev1beta1.MachineSet{}
	err := r.Get(ctx, req.NamespacedName, machineSet)
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

func (r *MachinesetReconciler) ProcessMachineSet(ctx context.Context, machineSet *machinev1beta1.MachineSet) (reconcile.Result, error) {
	// Get machines for machineset
	machines, err := m.GetMachinesForMachineSet(r.Client, machineSet)
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, machine := range machines {
		if machine.Status.NodeRef == nil || machine.Status.NodeRef.Name == "" {
			continue
		}
		node, err := m.GetNodeForMachine(r.Client, machine)
		if err != nil {
			klog.Errorf("failed to fetch node for machine %s", machine.Name)
			metrics.IncreaseNodeReconciliationFailure(machine.Name)
			return reconcile.Result{}, err
		}
		expectedLabels := r.getExpectedLabels(ctx, machineSet, machine, node)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Update labels in machine
		err = r.updateLabelsInMachine(ctx, machine, expectedLabels)
		if err != nil {
			metrics.IncreaseNodeReconciliationFailure(node.Name)
			return reconcile.Result{}, err
		}
		// Update taints in machine
		err = r.updateTaintsInMachine(ctx, machineSet, machine)
		if err != nil {
			metrics.IncreaseNodeReconciliationFailure(node.Name)
			return reconcile.Result{}, err
		}
		//Update labels in node
		err = r.updateLabelsInNode(ctx, node, expectedLabels)
		if err != nil {
			metrics.IncreaseNodeReconciliationFailure(node.Name)
			return reconcile.Result{}, err
		}
		// Update taints in node
		err = r.updateTaintsInNode(ctx, machine, node)
		if err != nil {
			metrics.IncreaseNodeReconciliationFailure(node.Name)
			if derr, ok := err.(DuplicateTaintError); ok {
				log.Log.Info("found duplicate taint on machine spec", "error", derr.Message)
				return reconcile.Result{Requeue: false}, nil
			}
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *MachinesetReconciler) getExpectedLabels(ctx context.Context, machineSet *machinev1beta1.MachineSet, machine *machinev1beta1.Machine, node *corev1.Node) map[string]string {
	result := machineSet.Spec.Template.Spec.Labels

	currentAnnotationValue := node.Annotations["managed.openshift.com/customlabels"]
	// Labels that are already set at the Node, but weren't set by the machine resource are ignored to avoid overwriting them
	for label := range machineSet.Spec.Template.Spec.Labels {
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

func (r *MachinesetReconciler) updateLabelsInMachine(ctx context.Context, m *machinev1beta1.Machine, expectedLabels map[string]string) error {
	if reflect.DeepEqual(expectedLabels, m.Spec.Labels) {
		return nil
	}
	m.Spec.Labels = expectedLabels
	err := r.Update(ctx, m)
	if err != nil {
		klog.Errorf("failed to update label in %s", m.Name)
		return err
	}
	return nil
}

// updateTaintsInMachine compares taints of machineset vs machine and updates them if they're not the same
func (r *MachinesetReconciler) updateTaintsInMachine(ctx context.Context, machineSet *machinev1beta1.MachineSet, machine *machinev1beta1.Machine) error {
	if !reflect.DeepEqual(machineSet.Spec.Template.Spec.Taints, machine.Spec.Taints) {
		machine.Spec.Taints = machineSet.Spec.Template.Spec.Taints
		if err := r.Update(ctx, machine); err != nil {
			return fmt.Errorf("failed to update taint for machine %s: %w", machine.Name, err)
		}
	}

	return nil
}

func (r *MachinesetReconciler) updateLabelsInNode(ctx context.Context, node *corev1.Node, expectedLabels map[string]string) error {
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
		node.Labels[newKey] = newVal
		if newAnnotationValue != "" {
			newAnnotationValue += ","
		}
		newAnnotationValue += newKey
	}
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	node.Annotations["managed.openshift.com/customlabels"] = newAnnotationValue

	err := r.Update(ctx, node)
	if err != nil {
		klog.Errorf("failed to update label in %s", node.Name)
		return err
	}

	return nil
}

// updateTaintsInNode ensures all taints on a node are expected. Expected taints on a node are any taints specified
// on a machine as well as any other NoSchedule taints.
func (r *MachinesetReconciler) updateTaintsInNode(ctx context.Context, machine *machinev1beta1.Machine, node *corev1.Node) error {
	noScheduleTaint := &corev1.Taint{
		Key:    corev1.TaintNodeUnschedulable,
		Effect: corev1.TaintEffectNoSchedule,
	}

	expectedTaints, duplicateTaintErr := CheckDuplicateTaints(machine.Spec.Taints)
	// expectedTaints := machine.Spec.Taints
	for _, taint := range node.Spec.Taints {
		if taint.MatchTaint(noScheduleTaint) {
			expectedTaints = append(expectedTaints, taint)
		}
	}

	// If there are any differences between expected taints and the node taints, update them
	toAdd, toRemove := TaintSliceDiff(expectedTaints, node.Spec.Taints)
	if len(toAdd) > 0 || len(toRemove) > 0 {
		node.Spec.Taints = expectedTaints
		if err := r.Update(ctx, node); err != nil {
			return fmt.Errorf("failed to update taints for node %s: %w", node.Name, err)
		}
	}

	return duplicateTaintErr
}

func CheckDuplicateTaints(taints []corev1.Taint) ([]corev1.Taint, error) {
	var err error = nil
	tmpTaints := make(map[corev1.Taint]bool, len(taints))
	uniqueTaints := make([]corev1.Taint, 0)
	for _, taint := range taints {
		if _, value := tmpTaints[taint]; !value {
			tmpTaints[taint] = true
			uniqueTaints = append(uniqueTaints, taint)
		} else {
			err = DuplicateTaintError{Message: fmt.Sprintf("duplicate taint in machine spec found - will be ignored: %v", taint)}
		}
	}
	return uniqueTaints, err
}

// TaintExists checks if the given taint exists in list of taints. Returns true if exists false otherwise.
func TaintExists(taints []corev1.Taint, taintToFind *corev1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

// TaintSliceDiff finds the difference between two taint slices and
// returns all new and removed elements of the new slice relative to the old slice.
// for example:
// input: expected=[a b] actual=[a c]
// output: taintsToAdd=[b] taintsToRemove=[c]
func TaintSliceDiff(expected, actual []corev1.Taint) ([]*corev1.Taint, []*corev1.Taint) {
	var (
		taintsToAdd    []*corev1.Taint
		taintsToRemove []*corev1.Taint
	)

	for i := range expected {
		if !TaintExists(actual, &expected[i]) {
			taintsToAdd = append(taintsToAdd, &expected[i])
		}
	}

	for i := range actual {
		if !TaintExists(expected, &actual[i]) {
			taintsToRemove = append(taintsToRemove, &actual[i])
		}
	}

	return taintsToAdd, taintsToRemove
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachinesetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&machinev1beta1.MachineSet{}).
		Named("machineset_controller").
		Complete(r)
}
