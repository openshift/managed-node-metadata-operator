package int_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	//nolint:typecheck // this import is used for general prettines, and currently kept
	. "github.com/openshift/managed-node-metadata-operator/int"
)

var (
	i *Integration
)
var _ = BeforeSuite(func() {
	var err error
	i, err = NewIntegration()
	Expect(err).NotTo(HaveOccurred())
	err = i.DisableWebhook()
	Expect(err).NotTo(HaveOccurred())
})
var _ = AfterSuite(func() {
	i.Shutdown()
})
var _ = Describe("Integrationtests", func() {

	Context("When adding a label to a MachineSet", func() {
		var (
			TestLabel string
			TestValue string
		)
		Context("When the labels doesn't exist on the Node", func() {
			BeforeEach(func() {
				TestLabel = "Fake-Node-Label"
				TestValue = "Fake-Node-Label-Value"
			})
			It("Is applied to the Nodes and Machines of the MachineSet", func() {
				workers, err := i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())
				workers.Spec.Template.Spec.Labels = map[string]string{
					TestLabel: TestValue,
				}
				err = i.Client.Update(context.TODO(), &workers)
				Expect(err).NotTo(HaveOccurred())
				time.Sleep(10 * time.Second)
				Expect(err).ToNot(HaveOccurred())
				machines, err := i.GetMachinesForMachineSets(&workers)
				Expect(err).ToNot(HaveOccurred())
				for _, machine := range machines {
					label, ok := machine.Spec.Labels[TestLabel]
					Expect(ok).To(BeTrue())
					Expect(label).To(Equal(TestValue))

					node, err := i.GetNodeForMachine(machine)
					Expect(err).NotTo(HaveOccurred())
					label, ok = node.Labels[TestLabel]
					Expect(ok).To(BeTrue())
					Expect(label).To(Equal(TestValue))
				}
			})
		})
	})
})
