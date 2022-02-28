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

	machinev1 "github.com/openshift/api/machine/v1beta1"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Map Machine changes to MachineSets using ControllerRef.
	err = c.Watch(
		&source.Kind{Type: &machinev1.Machine{}},
		&handler.EnqueueRequestForOwner{IsController: true, OwnerType: &machinev1.MachineSet{}},
	)
	if err != nil {
		return err
	}

	// Map Machine changes to MachineSets by machining labels.
	return c.Watch(
		&source.Kind{Type: &machinev1.Machine{}},
		handler.EnqueueRequestsFromMapFunc(mapFn),
	)
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

	for msKey, msValue := range machineSet.Labels {
		presentInMachine := false

		for idx := range allMachines.Items {
			machine := &allMachines.Items[idx]

			for mKey, mValue := range machine.Labels {
				if mKey == msKey && mValue == msValue {
					presentInMachine = true
					break
				}
			}

			if !presentInMachine {
				machine.Labels[msKey] = msValue
			}

			node := &v1.Node{}
			key := client.ObjectKey{Namespace: metav1.NamespaceNone, Name: machine.Status.NodeRef.Name}
			err := r.Client.Get(context.TODO(), key, node)
			if err != nil {
				fmt.Errorf("failed to fetch node for machine %s", machine.Name)
				return reconcile.Result{}, err
			}

			for mKey, mValue := range machine.Labels {
				presentInNode := false

				for nKey, nValue := range node.Labels {
					if nKey == mKey && nValue == mValue {
						presentInNode = true
						break
					}
				}

				if !presentInNode {
					node.Labels[mKey] = mValue
				}
			}
		}

	}

	return reconcile.Result{}, nil
}
