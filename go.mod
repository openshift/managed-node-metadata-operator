module github.com/openshift/managed-node-metadata-operator

go 1.16

require (
	github.com/golang/mock v1.6.0
	github.com/openshift/machine-api-operator v0.2.1-0.20211203013047-383c9b959b69
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/controller-runtime v0.9.6

)

replace sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20210622023641-c69a3acaee27

replace sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20211202014309-184ccedc799e
