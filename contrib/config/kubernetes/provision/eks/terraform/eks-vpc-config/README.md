## VPC + eksctl Config

This code creates a VPC that can be used for EKS, and creates a eksctl configuration file (see [eksctl/README.md](../../eksctl/README.md)).

## Instructions

The process to create this is the following:

1. create `terraform.tfvars` to specify eks and vpc names.
2. initialize modules and providers
3. create VPC infrastructure
4. create eksctl configuration file

You can do this with something liek the following below:

```bash
cat <-EOF > terraform.tfvars
region           = "us-east-2"
name             = "<my-vpc-name>"
eks_cluster_name = "<my-eks-cluster-name>"
EOF

terraform init
## create vpc infrastructure (pub/private subnets, route tables, internet gateway)
terraform apply --target module.vpc
## create configuration file based on existing VPC
terraform apply --target module.config
```
