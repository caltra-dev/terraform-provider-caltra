# Caltra Terraform provider

Manage settings for an existing Caltra organization, system-owned hosted agents and an organization default agent assignment with Terraform.

The provider source address is `caltra-dev/caltra`. Set `CALTRA_API_TOKEN` in the Terraform process environment to a scoped management token. Use a read-only token when a workflow only needs to plan. The API URL defaults to `https://api.caltra.dev` and can be set with `CALTRA_API_URL` or the provider's `api_url` argument.

See the [provider reference](docs/index.md) and the [configuration example](examples/hosted-agent/main.tf).

## Development

Use Go 1.25.8 or later. Terraform CLI runs the provider lifecycle tests against a local HTTP fixture.

```sh
go test ./...
TF_ACC=1 TF_ACC_TERRAFORM_PATH="$(command -v terraform)" go test ./internal/provider -run '^TestTerraform' -count=1
```

To use a development build with standalone Terraform, build the binary with `go build .`, then configure a [development override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) for `caltra-dev/caltra`.

## Release

The GitHub release workflow publishes version tags from `main`. Configure `GPG_PRIVATE_KEY` and `PASSPHRASE` for the release signing key. Register the matching public key and the GitHub repository on the public Terraform Registry for the first release. Each release includes Linux, macOS and Windows provider archives, build dependency metadata, the Registry manifest and GPG-signed SHA-256 checksums.

After publishing, verify a fresh `terraform init` using the exact new version and commit the resulting lock file in each consuming Terraform root. Follow the [Terraform Registry publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing).
