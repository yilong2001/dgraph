#####################################################################
# variables
#####################################################################
variable "region" {}
variable "name" {}
variable "eks_cluster_name" {}

#####################################################################
# Modules
#####################################################################
module "vpc" {
  source           = "../modules/eks-vpc"
  name             = var.name
  region           = var.region
  eks_cluster_name = var.eks_cluster_name
}

module "config" {
  source          = "../modules/eksctl-config"
  name            = var.eks_cluster_name
  region          = var.region
  vpc_id          = module.vpc.vpc_id
  instance_type   = "m5.2xlarge"
  public_key_name = "joaquin"
  filename        = "${path.module}/../../eksctl/cluster_config.yaml"
}
