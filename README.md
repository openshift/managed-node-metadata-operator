# Node Metadata Operator for OpenShift Dedicated

[![codecov](https://codecov.io/gh/openshift/managed-node-metadata-operator/branch/master/graph/badge.svg)](https://codecov.io/gh/openshift/managed-node-metadata-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/openshift/managed-node-metadata-operator)](https://goreportcard.com/report/github.com/openshift/managed-node-metadata-operator)
[![GoDoc](https://godoc.org/github.com/openshift/managed-node-metadata-operator?status.svg)](https://pkg.go.dev/mod/github.com/openshift/managed-node-metadata-operator)
[![License](https://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)

Directional metadata sync operator from MachineSets to existing Nodes

## General Overview

Adding node labels and taints to non-default MachinePools is allowed through OCM; however, due to [intentional limitations](https://github.com/openshift/machine-api-operator/blob/master/FAQ.md#adding-annotations-and-labels-to-nodes-via-machines) in OpenShift’s [machine-api-operator](https://github.com/openshift/machine-api-operator), labels and taints are not reconciled to existing machines within a machine set. Today you must scale down the MachinePool to 0 and back up again to update nodes. This is obviously undesirable and doesn’t present a good user experience for customers.

Managed OpenShift does not allow customers to label nodes directly. 

This managed-node-metadata-operator will attempt to watch MachineSet objects and reconcile any labels or taints that are added to the corresponding Nodes within the pool.

```mermaid
flowchart TD
    A[User updates MachineSet in OCM] --> B[Hive applies changes to MachineSet on cluster]
    B --> C;
    subgraph Managed Node Metadata Operator
      C[MNMO Picks up change to MachineSet and begins reconcile] --> D[Loop through all Machines in Machineset and Sync Label/Taint changes];
        D --> E[Remove Taints/Labels not present in MachineSet and present on Machine];
      subgraph Per Machine
        E --> F[Apply Taints/Labels present in MachineSet and not present on Machine];
        F --> G[Get referenced Node]
        G --> H[Remove Taints/Labels not present in Machine and present on Node];
        H --> I[Apply Taints/Labels present in Machine and not present on Node];
      end
    end
```

## Development and Testing
Please refer to the [development and testing guide](docs/development-and-testing.md).

## Boilerplate
This repository subscribes to the [openshift/golang-osd-operator](https://github.com/openshift/boilerplate/tree/master/boilerplate/openshift/golang-osd-operator) convention of [boilerplate](https://github.com/openshift/boilerplate/).
See the [README](boilerplate/openshift/golang-osd-operator/README.md) for details about the functionality that brings in.


