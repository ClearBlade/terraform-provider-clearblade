# ClearBlade Terraform provider

A modern (protocol 6) Terraform provider for the ClearBlade IoT Core service (work-in-progress)

## `Development status`

This Terraform provider code for the ClearBlade IoT Core service is currently in preview.

## Getting started

Add this provider to your Terraform configuration block:

```terraform
terraform {
  required_providers {
    clearblade = {
      source = "clearblade/clearblade"
      version = "0.0.0-beta.4"
    }
  }
}

# Configure the ClearBlade provider
provider "clearblade" {

}
```
