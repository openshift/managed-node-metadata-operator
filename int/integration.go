package int

import (
	"context"
	"fmt"
	"time"

	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Integration is an integration testing toolset, providing utilities to get resources from a cluster
type Integration struct {
	Client client.Client
	mgr    manager.Manager
}

// NewIntegration creates a new integration testing toolset, providing utilities to get resources from a cluster
func NewIntegration() (*Integration, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(machinev1.AddToScheme(scheme))
	utilruntime.Must(admissionregistration.AddToScheme(scheme))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: ":9999",
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

// GetWorkerMachineSet returns the MachineSet with the worker label
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
		role, ok = ms.Labels["machine.openshift.io/cluster-api-machine-role"]
		if ok && role == "worker" {
			return ms, nil
		}

	}
	return machinev1.MachineSet{}, fmt.Errorf("no worker MachineSet found")
}

// DisableWebhook removes the sre-regular-user-validation webhook, preventing edits to MachineSets
func (i *Integration) DisableWebhook() error {
	webhook := admissionregistration.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sre-regular-user-validation",
		},
	}
	err := i.Client.Delete(context.TODO(), &webhook)
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}
