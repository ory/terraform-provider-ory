# Security Policy

## Reporting Vulnerabilities

If you discover a security vulnerability, please report it responsibly by emailing [security@ory.sh](mailto:security@ory.sh). Do **not** open a public GitHub issue for security vulnerabilities.

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

### Write-only secrets

Some secrets are **write-only**: the provider sends them to the Ory API on create
and update, but the API does not return them in its responses, so the provider never
reads them back from the API. The value configured in Terraform is the source of
truth (and is still stored in state, masked as sensitive). Because the provider does
not refresh these from the API, it tolerates the API omitting the value or returning
a masked sentinel (such as `****`) without producing a spurious diff:

- `smtp_connection_uri` in `ory_project_config`
- `client_secret` and `apple_private_key` in `ory_social_provider`

These values still originate from your configuration, so keep them in sensitive
variables and protect your state as described above.
