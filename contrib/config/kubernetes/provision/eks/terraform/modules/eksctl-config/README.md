# Config for eksctl

This creates an configuration file for use with `eksctl`.

## Examples

```terraform
module "config" {
  source             = "."
  name               = "devtest-cluster"
  region             = "us-west-2"
  vpc_id             = "vpc-xxxxxxxx"
  instance_type      = "m5.2xlarge"
  public_key_name    = "my-org-ssh-key"
  filename           = "${path.module}/cluster_config.yaml"
}
```
