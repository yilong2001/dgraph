# EKS VPC

This creates an EKS compatible VPC infrastructure with 3 private subnets and 3 public subnets.

## Examples

This is an example where the VPC tag `Name` the same as the future eks-cluster name.

```terraform
module "eks-vpc" {
  source = "."
  name   = "devtest-cluster"
  region = "us-west-2"
}
```

This is an example where the VPC tag `Name` is different that the future eks-cluster name.

```terraform
module "eks-vpc" {
  source = "."
  name   = "devtest-vpc"
  region = "us-west-2"
  eks_cluster_name = "devtest-cluster"
}
```
