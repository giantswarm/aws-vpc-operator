[![CircleCI](https://circleci.com/gh/giantswarm/aws-vpc-operator.svg?style=shield)](https://circleci.com/gh/giantswarm/aws-vpc-operator)

# aws-vpc-operator

Operator for managing AWS VPCs for Cluster API AWS workload clusters.

We use this operator to reconcile AWSClusters using the private VPC mode, because in those cases we need a custom VPC, different from what CAPA would create.
Clusters choose the private VPC mode using this annotation on the `AWSCluster` CR:

```yaml
aws.giantswarm.io/vpc-mode: private
```
