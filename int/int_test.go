package int_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	machinev1 "github.com/openshift/machine-api-operator/pkg/apis/machine/v1beta1"
	. "github.com/openshift/managed-node-metadata-operator/int"
	m "github.com/openshift/managed-node-metadata-operator/pkg/machine"
)

var (
	i *Integration
)

const (
	MaxWaitTime time.Duration = 30 * time.Second
)

var _ = BeforeSuite(func() {
	var err error
	i, err = NewIntegration()
	Expect(err).NotTo(HaveOccurred())
	err = i.DisableWebhook()
	Expect(err).NotTo(HaveOccurred())
})

func setMachineSetLabel(machineset machinev1.MachineSet, label string, value string) {
	machineset.Spec.Template.Spec.Labels = map[string]string{
		label: value,
	}
	err := i.Client.Update(context.TODO(), &machineset)
	Expect(err).NotTo(HaveOccurred())
}

func removeMachineSetLabel(machineset machinev1.MachineSet, label string) {
	delete(machineset.Spec.Template.Spec.Labels, label)
	err := i.Client.Update(context.TODO(), &machineset)
	Expect(err).NotTo(HaveOccurred())
}

func waitForNodeLabel(machineset machinev1.MachineSet, label string, value string) {
	lastFailure := ""
WAIT:
	for t := 0 * time.Second; t < MaxWaitTime; t = t + 1*time.Second {
		time.Sleep(1 * time.Second)
		machines, err := m.GetMachinesForMachineSet(i.Client, &machineset)
		Expect(err).ToNot(HaveOccurred())
		allMachinesOk := true
		for _, machine := range machines {
			machinelabelvalue, ok := machine.Spec.Labels[label]
			if !ok {
				allMachinesOk = false
				lastFailure = "machine/" + machine.Name
				continue WAIT
			}
			Expect(machinelabelvalue).To(Equal(value))

			node, err := m.GetNodeForMachine(i.Client, machine)
			Expect(err).NotTo(HaveOccurred())
			nodelabelvalue, ok := node.Labels[label]
			if !ok {
				allMachinesOk = false
				lastFailure = "node/" + node.Name
				continue WAIT
			}
			Expect(nodelabelvalue).To(Equal(value))
		}
		if allMachinesOk {
			return
		}
	}
	Fail("Label '" + label + "' did not get the expected value '" + value + "' after " + MaxWaitTime.String() + " on " + lastFailure)
}

func waitForNodeLabelAbsence(machineset machinev1.MachineSet, label string) {
	lastFailure := ""
WAIT:
	for t := 0 * time.Second; t < MaxWaitTime; t = t + 1*time.Second {
		time.Sleep(1 * time.Second)
		machines, err := m.GetMachinesForMachineSet(i.Client, &machineset)
		Expect(err).ToNot(HaveOccurred())
		allMachinesOk := true
		for _, machine := range machines {
			_, ok := machine.Spec.Labels[label]
			if ok {
				allMachinesOk = false
				lastFailure = "machine/" + machine.Name
				continue WAIT
			}

			node, err := m.GetNodeForMachine(i.Client, machine)
			Expect(err).NotTo(HaveOccurred())
			_, ok = node.Labels[label]
			if ok {
				allMachinesOk = false
				lastFailure = "node/" + node.Name
				continue WAIT
			}
		}
		if allMachinesOk {
			return
		}
	}
	Fail("Label '" + label + "' did not get removed as expected after " + MaxWaitTime.String() + " on " + lastFailure)
}

var _ = Describe("Integrationtests", func() {
	var (
		TestLabel string
		TestValue string
		workers   machinev1.MachineSet
	)
	BeforeEach(func() {
		var err error
		workers, err = i.GetWorkerMachineSet()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("When adding a label to a MachineSet", func() {
		Context("When the label doesn't exist on the Node", func() {
			BeforeEach(func() {
				TestLabel = "Fake-Node-Label"
				TestValue = "Fake-Node-Label-Value"

				//Make sure the label is not set before adding it
				removeMachineSetLabel(workers, TestLabel)
				waitForNodeLabelAbsence(workers, TestLabel)

				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())
			})

			It("Is applied to the Nodes and Machines of the MachineSet", func() {
				setMachineSetLabel(workers, TestLabel, TestValue)
				waitForNodeLabel(workers, TestLabel, TestValue)
			})

			AfterEach(func() {
				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())

				//Clean up
				removeMachineSetLabel(workers, TestLabel)
				waitForNodeLabelAbsence(workers, TestLabel)
			})

		})
	})

	Context("When removing a label from a MachineSet", func() {
		Context("When the label exists on the Node", func() {
			BeforeEach(func() {
				TestLabel = "Fake-Node-Label"
				TestValue = "Fake-Node-Label-Value"

				//Add Label and wait for it to appear, so we have something to remove
				setMachineSetLabel(workers, TestLabel, TestValue)
				waitForNodeLabel(workers, TestLabel, TestValue)

				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())
			})

			It("Is removed from Nodes and Machines of the MachineSet", func() {
				removeMachineSetLabel(workers, TestLabel)
				waitForNodeLabelAbsence(workers, TestLabel)
			})
		})
	})
})
