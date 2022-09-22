package int_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	. "github.com/openshift/managed-node-metadata-operator/int"
	m "github.com/openshift/managed-node-metadata-operator/pkg/machine"
)

var (
	i *Integration
)

const (
	MaxWaitTime = 30 * time.Second
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

func setMachineSetTaint(machineset machinev1.MachineSet, key string, value string) {
	machineset.Spec.Template.Spec.Taints = []v1.Taint{
		{
			Effect: v1.TaintEffectPreferNoSchedule,
			Value:  value,
			Key:    key,
		},
	}
	err := i.Client.Update(context.TODO(), &machineset)
	Expect(err).NotTo(HaveOccurred())
}

func setNodeLabel(machineset machinev1.MachineSet, label string, value string) {
	machines, err := m.GetMachinesForMachineSet(i.Client, &machineset)
	Expect(err).ToNot(HaveOccurred())
	Expect(len(machines)).To(BeNumerically(">", 0))
	for _, machine := range machines {
		node, err := m.GetNodeForMachine(i.Client, machine)
		Expect(err).ToNot(HaveOccurred())
		node.Labels[label] = value
		err = i.Client.Update(context.TODO(), node)
		Expect(err).NotTo(HaveOccurred())
	}
}

func cleanupMachineSetLabels(machineset machinev1.MachineSet) {
	machineset.Spec.Template.Spec.Labels = map[string]string{}
	err := i.Client.Update(context.TODO(), &machineset)
	Expect(err).NotTo(HaveOccurred())
}

func cleanupMachineSetTaint(machineset machinev1.MachineSet) {
	machineset.Spec.Template.Spec.Taints = []v1.Taint{}
	err := i.Client.Update(context.TODO(), &machineset)
	Expect(err).NotTo(HaveOccurred())
}

func removeMachineSetLabel(machineset machinev1.MachineSet, label string) {
	delete(machineset.Spec.Template.Spec.Labels, label)
	err := i.Client.Update(context.TODO(), &machineset)
	Expect(err).NotTo(HaveOccurred())
}

func removeMachineSetTaint(machineset machinev1.MachineSet, key string) {
	var newTaints []v1.Taint

	for i, taint := range machineset.Spec.Template.Spec.Taints {
		if taint.Key == key {
			continue
		}
		newTaints = append(newTaints, machineset.Spec.Template.Spec.Taints[i])
	}
	machineset.Spec.Template.Spec.Taints = newTaints
	err := i.Client.Update(context.TODO(), &machineset)
	Expect(err).NotTo(HaveOccurred())
}

func waitForNodeLabel(machineset machinev1.MachineSet, label string, value string, nodeOnly bool) {
	lastFailure := ""

	// Wait for a maximum of MaxWaitTime, if the timer goes off, mark the test as failed
	timer := time.NewTimer(MaxWaitTime)
	go func() {
		<-timer.C
		Fail("Label '" + label + "' did not get the expected value '" + value + "' after " + MaxWaitTime.String() + " on " + lastFailure)
	}()

	for {
		time.Sleep(1 * time.Second)
		machines, err := m.GetMachinesForMachineSet(i.Client, &machineset)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(machines)).To(BeNumerically(">", 0))
		allMachinesOk := true
		for _, machine := range machines {
			if !nodeOnly {
				machineLabelValue, ok := machine.Spec.Labels[label]
				if !ok {
					allMachinesOk = false
					lastFailure = "machine/" + machine.Name
					continue
				}
				Expect(machineLabelValue).To(Equal(value))

			}

			node, err := m.GetNodeForMachine(i.Client, machine)
			Expect(err).NotTo(HaveOccurred())
			nodeLabelValue, ok := node.Labels[label]
			if !ok {
				allMachinesOk = false
				lastFailure = "node/" + node.Name
				continue
			}
			Expect(nodeLabelValue).To(Equal(value))
		}
		if allMachinesOk {
			return
		}
	}
}

func waitForNodeTaint(machineset machinev1.MachineSet, key string, value string, nodeOnly bool) {
	lastFailure := ""

	// Wait for a maximum of MaxWaitTime, if the timer goes off, mark the test as failed
	timer := time.NewTimer(MaxWaitTime)
	go func() {
		<-timer.C
		Fail("Taint '" + key + "' did not get the expected value '" + value + "' after " + MaxWaitTime.String() + " on " + lastFailure)
	}()

	for {
		time.Sleep(1 * time.Second)
		machines, err := m.GetMachinesForMachineSet(i.Client, &machineset)
		Expect(err).ToNot(HaveOccurred())
		allMachinesOk := true
		for _, machine := range machines {
			if !nodeOnly {
				machineTaintKeyExist := false
				for _, taint := range machine.Spec.Taints {
					if taint.Key == key {
						machineTaintKeyExist = true
						Expect(taint.Value).To(Equal(value))
						break
					}
				}
				if machineTaintKeyExist == false {
					allMachinesOk = false
					lastFailure = "machine/" + machine.Name
					continue
				}
			}

			node, err := m.GetNodeForMachine(i.Client, machine)
			Expect(err).NotTo(HaveOccurred())
			nodeTaintKeyExist := false
			for _, taint := range node.Spec.Taints {
				if taint.Key == key {
					nodeTaintKeyExist = true
					Expect(taint.Value).To(Equal(value))
					break
				}
			}
			if nodeTaintKeyExist == false {
				allMachinesOk = false
				lastFailure = "node/" + node.Name
				continue
			}
		}
		if allMachinesOk {
			return
		}
	}
}

func waitForNodeLabelAbsence(machineset machinev1.MachineSet, label string) {
	lastFailure := ""

	// Wait for a maximum of MaxWaitTime, if the timer goes off, mark the test as failed
	timer := time.NewTimer(MaxWaitTime)
	go func() {
		<-timer.C
		Fail("Label '" + label + "' did not get removed as expected after " + MaxWaitTime.String() + " on " + lastFailure)
	}()

	for {
		time.Sleep(1 * time.Second)
		machines, err := m.GetMachinesForMachineSet(i.Client, &machineset)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(machines)).To(BeNumerically(">", 0))
		allMachinesOk := true
		for _, machine := range machines {
			_, ok := machine.Spec.Labels[label]
			if ok {
				allMachinesOk = false
				lastFailure = "machine/" + machine.Name
				continue
			}

			node, err := m.GetNodeForMachine(i.Client, machine)
			Expect(err).NotTo(HaveOccurred())
			_, ok = node.Labels[label]
			if ok {
				allMachinesOk = false
				lastFailure = "node/" + node.Name
				continue
			}
		}
		if allMachinesOk {
			return
		}
	}
}

func waitForNodeTaintAbsence(machineset machinev1.MachineSet, key string) {
	lastFailure := ""

	// Wait for a maximum of MaxWaitTime, if the timer goes off, mark the test as failed
	timer := time.NewTimer(MaxWaitTime)
	go func() {
		<-timer.C
		Fail("Taint '" + key + "' did not get removed as expected after " + MaxWaitTime.String() + " on " + lastFailure)
	}()

	for {
		time.Sleep(1 * time.Second)
		machines, err := m.GetMachinesForMachineSet(i.Client, &machineset)
		Expect(err).ToNot(HaveOccurred())
		allMachinesOk := true
		for _, machine := range machines {
			for _, taint := range machine.Spec.Taints {
				if taint.Key == key {
					allMachinesOk = false
					lastFailure = "machine/" + machine.Name
					continue
				}
			}

			node, err := m.GetNodeForMachine(i.Client, machine)
			Expect(err).NotTo(HaveOccurred())
			for _, taint := range node.Spec.Taints {
				if taint.Key == key {
					allMachinesOk = false
					lastFailure = "node/" + node.Name
					continue
				}
			}
		}
		if allMachinesOk {
			return
		}
	}
}

var _ = Describe("Integrationtests", func() {
	var (
		TestLabel      string
		TestTaint      string
		TestValue      string
		TestValueTaint string
		workers        machinev1.MachineSet
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
				cleanupMachineSetLabels(workers)
				waitForNodeLabelAbsence(workers, TestLabel)

				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())
			})

			It("Is applied to the Nodes and Machines of the MachineSet", func() {
				setMachineSetLabel(workers, TestLabel, TestValue)
				waitForNodeLabel(workers, TestLabel, TestValue, false)
			})

			AfterEach(func() {
				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())

				//Clean up
				cleanupMachineSetLabels(workers)
				waitForNodeLabelAbsence(workers, TestLabel)
			})

		})
		Context("When overriding a label of a Node", func() {
			BeforeEach(func() {
				TestLabel = "Fake-Node-Label"
				TestValue = "Fake-Node-Label-Value"

				//Make sure the label is not set before adding it
				cleanupMachineSetLabels(workers)
				waitForNodeLabelAbsence(workers, TestLabel)

				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())
			})
			It("Doesn't change", func() {
				workers.Spec.Template.Spec.Labels = map[string]string{
					TestLabel:                        TestValue,
					"node-role.kubernetes.io/worker": "overruled",
				}
				err := i.Client.Update(context.TODO(), &workers)
				Expect(err).NotTo(HaveOccurred())
				waitForNodeLabel(workers, TestLabel, TestValue, false)
				waitForNodeLabel(workers, "node-role.kubernetes.io/worker", "", true)
			})
			AfterEach(func() {
				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())

				//Clean up
				cleanupMachineSetLabels(workers)
				waitForNodeLabelAbsence(workers, TestLabel)
				setNodeLabel(workers, "node-role.kubernetes.io/worker", "")
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
				waitForNodeLabel(workers, TestLabel, TestValue, false)

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

	Context("When adding a taint to a MachineSet", func() {
		Context("When the taint doesn't exist on the Node", func() {
			BeforeEach(func() {
				TestTaint = "Fake-Node-Taint"
				TestValueTaint = "Fake-Node-Taint-Value"

				//Make sure the taint is not set before adding it
				cleanupMachineSetTaint(workers)
				waitForNodeTaintAbsence(workers, TestTaint)

				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())
			})

			It("Is applied to the Nodes and Machines of the MachineSet", func() {
				setMachineSetTaint(workers, TestTaint, TestValueTaint)
				waitForNodeTaint(workers, TestTaint, TestValueTaint, false)
			})

			AfterEach(func() {
				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())

				//Clean up
				cleanupMachineSetTaint(workers)
				waitForNodeTaintAbsence(workers, TestTaint)
			})
		})
	})

	Context("When removing a taint from a MachineSet", func() {
		Context("When the taint exists on the Node", func() {
			BeforeEach(func() {
				TestTaint = "Fake-Node-Taint"
				TestValueTaint = "Fake-Node-Taint-Value"

				//Add Taint and wait for it to appear, so we have something to remove
				setMachineSetTaint(workers, TestTaint, TestValueTaint)
				waitForNodeTaint(workers, TestTaint, TestValueTaint, false)

				//refresh workers
				var err error
				workers, err = i.GetWorkerMachineSet()
				Expect(err).NotTo(HaveOccurred())
			})

			It("Is removed from Nodes and Machines of the MachineSet", func() {
				removeMachineSetTaint(workers, TestTaint)
				waitForNodeTaintAbsence(workers, TestTaint)
			})
		})
	})
})
