# Security Policy

## Reporting Vulnerabilities

If you discover a security vulnerability, please report it responsibly by emailing [security@ory.com](mailto:security@ory.com). Do **not** open a public GitHub issue for security vulnerabilities.

We will acknowledge receipt within 3 business days and work with you to understand and address the issue.

## Security Best Practices

When using this provider:

1. **Protect API Keys**: Never commit API keys to version control. Use environment variables or a secrets manager.

2. **Use Terraform State Encryption**: Enable encryption for your Terraform state, especially in remote backends.

3. **Restrict State Access**: Limit who can access Terraform state files, as they may contain sensitive values.

4. **Review Plans**: Always review `terraform plan` output before applying changes.

5. **Audit Changes**: Use version control and code review for all Terraform configuration changes.

## Known Security Considerations

- `client_secret` values in `ory_oauth2_client` are stored in Terraform state
- `password` values in `ory_identity` are stored in Terraform state
- SMTP connection URIs may contain credentials
- API keys configured in the provider are passed to the Ory API

Use Terraform's sensitive variable handling and state encryption to protect these values.

### Not-read-back secrets (still stored in state)

Some secrets are sent to the Ory API on create and update, but the API does not
return them in its responses, so the provider never reads them back. The value
configured in Terraform is the source of truth and is **still stored in state**,
masked as sensitive. Because the provider does not refresh these from the API, it
tolerates the API omitting the value or returning a masked sentinel (such as
`****`) without producing a spurious diff:

- `smtp_connection_uri` in `ory_project_config`
- `client_secret` and `apple_private_key` in `ory_social_provider`
- `password` in `ory_identity`
- `client_secret` in `ory_oauth2_client` (server-generated; returned only on create, never on subsequent reads)

These values still originate from your configuration (or, for `ory_oauth2_client.client_secret`, from the create response), so keep them in sensitive
variables and protect your state as described above.

### Write-only arguments (never stored in state)

For stronger protection, several secrets also offer **write-only** arguments
(Terraform 1.11+ [write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral/write-only)).
A write-only value is sent to the Ory API but is **never written to Terraform state
or plan** — making it ideal for credentials sourced from an ephemeral resource such
as a Vault secret. Each write-only argument is mutually exclusive with its stateful
counterpart and has a companion `*_wo_version` attribute; because write-only values
are not stored, Terraform cannot diff them, so you change the version whenever the
secret rotates to have the provider re-send it.

| Resource | Write-only argument | Stateful counterpart |
|----------|---------------------|----------------------|
| `ory_social_provider` | `client_id_wo`, `client_secret_wo`, `apple_private_key_wo` | `client_id`, `client_secret`, `apple_private_key` |
| `ory_identity` | `password_wo` | `password` |
| `ory_action` | `webhook_auth_basic_auth_password_wo`, `webhook_auth_api_key_value_wo` | `webhook_auth_basic_auth_password`, `webhook_auth_api_key_value` |
| `ory_oauth2_client` | `jwks_wo` | `jwks` |
| `ory_project_config` | `smtp_connection_uri_wo` | `smtp_connection_uri` |

> **Note:** Write-only values are only available to the provider during create and
> update (Terraform does not expose them during refresh or import). Importing a
> resource therefore cannot recover a write-only value — re-supply it in your
> configuration after import.
