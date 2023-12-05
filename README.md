<div align="center">
   <p>ClearBlade Terraform provider</p>
   <a href="https://clearblade.atlassian.net/wiki/spaces/IC/overview"><img src="https://img.shields.io/static/v1?label=Docs&message=API Ref&color=000000&style=for-the-badge" /></a>
   <a href="https://clearblade.atlassian.net/wiki/spaces/IC/pages/2399109225/ClearBlade+Terraform+provider"><img src="https://img.shields.io/static/v1?label=Docs&message=SDK&color=003300&style=for-the-badge" /></a>
  <a href="https://opensource.org/licenses/MPL-2.0"><img src="https://img.shields.io/badge/License-MPL-blue.svg?style=for-the-badge" /></a>
</div>

## Authentication

To make API requests, you must create or download a ClearBlade service account credential within your [ClearBlade IoT Core Developer Portal](https://iot.clearblade.com/iot-core/). Once you are in the Developer Portal, [add service accounts to a project](https://clearblade.atlassian.net/wiki/spaces/IC/pages/2240675843/) and download the JSON file with your service account's credentials.

Use the provider block to configure the path to your service account's JSON file for authentication. The following environment variable can be used to set your credentials in your terminal or IDE environment:

```
 export CLEARBLADE_CONFIGURATION=/path/to/file.json
```

<!-- Start SDK Installation -->

## SDK installation

Copy and paste this code into your Terraform configuration to install this provider, then, run `terraform init`.

```hcl
terraform {
  required_providers {
    clearblade = {
      source = "ClearBlade/clearblade"
      version = "x.y.z" # Check out the latest version in the release section
    }
  }
}

provider "clearblade" {
  # Configuration options
  credentials = "path-to-clearblade-credentials-auth-file"
  project     = "your-gcp-project-id"
  region      = "your-gcp-region"
}
```

<!-- End SDK Installation -->

## Testing the provider locally

Terraform allows you to use local provider builds by setting a `dev_overrides` block in a configuration file called `.terraformrc`. This block overrides all other configured installation methods.

Terraform searches for the `.terraformrc` file in your home directory and applies any configuration settings you set.

```
provider_installation {

  dev_overrides {
      "registry.terraform.io/ClearBlade/clearblade" = "<PATH>"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and no other providers will be available.
  direct {}
}
```

Your `<PATH>` may vary depending on how your Go environment variables are configured. Execute `go env GOBIN` to set them, then set the `<PATH>` to the value returned. If nothing is returned, set it to the default location, `$HOME/go/bin`.

## Creating a new release

To create a new release, execute the following commands:

```shell
# Use sem var for tags, i.e. v0.0.1
git tag [tag]
git push origin [tag]
```

### Contributions

While we value open-source contributions to this SDK, this library is generated programmatically.
Feel free to open a PR or a Github issue as a proof of concept, and we'll do our best to include it in a future release!
