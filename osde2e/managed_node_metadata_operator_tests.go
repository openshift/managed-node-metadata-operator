//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	"fmt"
	"os"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	clustersmgmtv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/managed-node-metadata-operator/config"
	"github.com/openshift/osde2e-common/pkg/clients/ocm"
	"github.com/openshift/osde2e-common/pkg/clients/openshift"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var _ = ginkgo.Describe("managed-node-metadata-operator", ginkgo.Ordered, func() {
	var (
		k8s              *openshift.Client
		ocmClusterClient *clustersmgmtv1.ClusterClient
		machinepoolName  string
		machineSetName   string
		clusterID        string
	)

	ginkgo.BeforeAll(func(ctx context.Context) {
		clusterID = os.Getenv("OCM_CLUSTER_ID")
		Expect(clusterID).ShouldNot(BeEmpty(), "OCM_CLUSTER_ID is required but not set")

		ocmConn, err := ocm.New(ctx, os.Getenv("OCM_TOKEN"), ocm.Stage)
		Expect(err).ShouldNot(HaveOccurred(), "unable to setup ocm client")
		ginkgo.DeferCleanup(ocmConn.Connection.Close)

		// create the specific cluster OCM client
		ocmClusterClient = ocmConn.ClustersMgmt().V1().Clusters().Cluster(clusterID)
		clusterResp, err := ocmClusterClient.Get().SendContext(ctx)
		Expect(err).Should(BeNil(), "unable to fetch cluster %s", clusterID)

		cluster := clusterResp.Body()
		machinepoolReplicaCount := 1
		if cluster.MultiAZ() {
			machinepoolReplicaCount = 3
		}
		log.SetLogger(ginkgo.GinkgoLogr)
		k8s, err = openshift.New(ginkgo.GinkgoLogr)
		Expect(err).ShouldNot(HaveOccurred(), "unable to setup k8s client")

		machinepoolName = envconf.RandomName("osde2e", 10)
		var instanceType = "m5.xlarge"
		if cluster.CloudProvider().ID() == "gcp" {
			// https://docs.openshift.com/dedicated/osd_architecture/osd_policy/osd-service-definition.html#gcp-compute-types_osd-service-definition
			// In osde2e, this is a non-CCS OSD GCP cluster with very limited instance type support
			instanceType = "custom-4-16384"
		}
		machinepoolBuilder := clustersmgmtv1.NewMachinePool().ID(machinepoolName).InstanceType(instanceType).Replicas(machinepoolReplicaCount)
		machinepool, err := machinepoolBuilder.Build()
		Expect(err).Should(BeNil(), "machinepoolBuilder.Build failed")
		_, err = ocmClusterClient.MachinePools().Add().Body(machinepool).SendContext(ctx)
		Expect(err).Should(BeNil(), "failed to create machinepool")

		// delete the pool at the end
		ginkgo.DeferCleanup(ocmClusterClient.MachinePools().MachinePool(machinepoolName).Delete().SendContext)

		// wait for it to be ready
		err = wait.For(func() (bool, error) {
			lblSel := resources.WithLabelSelector(labels.FormatLabels(map[string]string{"hive.openshift.io/machine-pool": machinepoolName}))
			var machineSetList machinev1beta1.MachineSetList
			if err = k8s.WithNamespace("openshift-machine-api").List(ctx, &machineSetList, lblSel); err != nil {
				return false, fmt.Errorf("unable to list machinesets: %w", err)
			}
			if len(machineSetList.Items) < 1 {
				return false, nil
			}
			machineSet := machineSetList.Items[0]
			machineSetName = machineSet.GetName()
			return machineSet.Status.ReadyReplicas == *machineSet.Spec.Replicas, nil
		}, wait.WithTimeout(600*time.Second))
		Expect(err).Should(BeNil(), "wait.For machinepool ready failed")
	})

	haveLabels := func(node corev1.Node, lbls map[string]string) bool {
		nodeLabels := node.GetObjectMeta().GetLabels()
		for key, expected := range lbls {
			got, ok := nodeLabels[key]
			if !ok || expected != got {
				return false
			}
		}
		return true
	}

	haveTaint := func(node corev1.Node, taintMap map[string]string) bool {
		nodeTaints := node.Spec.Taints
		if len(taintMap) == 0 {
			return len(nodeTaints) == 0
		}
		for _, taint := range nodeTaints {
			if taint.Key == taintMap["key"] {
				return taint.Value == taintMap["value"] && string(taint.Effect) == taintMap["effect"]
			}
		}
		return false
	}

	nodesTo := func(ctx context.Context, items map[string]string, have func(corev1.Node, map[string]string) bool) func() (bool, error) {
		return func() (bool, error) {
			lblSel := resources.WithLabelSelector(labels.FormatLabels(map[string]string{"machine.openshift.io/cluster-api-machineset": machineSetName}))
			var machineList machinev1beta1.MachineList
			if err := k8s.WithNamespace("openshift-machine-api").List(ctx, &machineList, lblSel); err != nil {
				return false, fmt.Errorf("unable to list machines: %w", err)
			}
			for _, machine := range machineList.Items {
				var node corev1.Node
				if err := k8s.Get(ctx, machine.Status.NodeRef.Name, "", &node); err != nil {
					return false, err
				}
				if !have(node, items) {
					return false, nil
				}
			}
			return true, nil
		}
	}

	ginkgo.DescribeTable("labels are synced", func(ctx context.Context, lbls map[string]string) {
		machinepool, err := clustersmgmtv1.NewMachinePool().ID(machinepoolName).Labels(lbls).Build()
		Expect(err).Should(BeNil(), "failed to build machinepool with labels")
		_, err = ocmClusterClient.MachinePools().MachinePool(machinepoolName).Update().Body(machinepool).SendContext(ctx)
		Expect(err).Should(BeNil(), "failed to update machinepool labels")
		Expect(wait.For(nodesTo(ctx, lbls, haveLabels), wait.WithTimeout(2*time.Minute))).Should(BeNil(), "waiting for labels to be synced failed")
	},
		ginkgo.Entry("added", map[string]string{"osde2e": "one"}),
		ginkgo.Entry("updated", map[string]string{"osde2e": "two"}),
		ginkgo.Entry("deleted", map[string]string{}),
	)

	ginkgo.DescribeTable("taints are synced", func(ctx context.Context, taintMap map[string]string) {
		var taintBuilders []*clustersmgmtv1.TaintBuilder
		if len(taintMap) > 0 {
			taintBuilders = append(taintBuilders, clustersmgmtv1.NewTaint().Key(taintMap["key"]).Value(taintMap["value"]).Effect(taintMap["effect"]))
		}
		machinepool, err := clustersmgmtv1.NewMachinePool().ID(machinepoolName).Taints(taintBuilders...).Build()
		Expect(err).Should(BeNil(), "failed to build machinepool with taints")
		_, err = ocmClusterClient.MachinePools().MachinePool(machinepoolName).Update().Body(machinepool).SendContext(ctx)
		Expect(err).Should(BeNil(), "failed to update machinepool taints")
		Expect(wait.For(nodesTo(ctx, taintMap, haveTaint), wait.WithTimeout(2*time.Minute))).Should(BeNil(), "waiting for taints to be synced failed")
	},
		ginkgo.Entry("added", map[string]string{"key": "osde2e", "value": "one", "effect": "NoSchedule"}),
		ginkgo.Entry("updated", map[string]string{"key": "osde2e", "value": "two", "effect": "NoExecute"}),
		ginkgo.Entry("deleted", map[string]string{}),
	)

	ginkgo.It("can be upgraded", func(ctx context.Context) {
		err := k8s.UpgradeOperator(ctx, config.OperatorName, config.OperatorNamespace)
		Expect(err).NotTo(HaveOccurred(), "operator upgrade failed")
	})

})
