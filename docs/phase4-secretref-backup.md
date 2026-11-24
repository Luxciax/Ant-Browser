# Phase 4: SecretRef backup integration

Backup credentials are local secrets and are not written to profile packages.

The local backup settings file now stores SecretRefs for:

- OpenList token;
- S3 access key ID;
- S3 secret access key;
- S3 session token.

On Windows the SecretStore encrypts each value with user-scoped DPAPI. On
non-Windows platforms the store requires a high-entropy
`ANT_BROWSER_SECRET_KEY` environment variable and uses AES-GCM. If the
protection provider is unavailable or encryption fails, saving fails: Ant does
not fall back to writing a new plaintext credential.

Legacy plaintext `backup.local.yaml` values remain readable only as migration
input. A successful settings save rewrites them as SecretRefs.

`config.yaml` continues to use the existing backup sanitization path and does
not contain backup credentials.

Profile-package manifests explicitly declare local SecretRefs as non-portable
and backup credentials as excluded. Extension artifacts and their bindings
remain portable through the existing extension package mechanism.

The LaunchServer API key uses the same SecretStore. New rotations refuse to
write a key when the protection provider is unavailable. Existing plaintext
API keys are treated only as legacy migration input; when SecretStore is
available they are migrated to `api_key_ref`, and subsequent config writes
omit the plaintext key.

Secret updates use a staged-ref commit: new encrypted entries are created
first, configuration is atomically written to point at them, and only then are
the old refs deleted. If config serialization/write fails, the staged refs are
deleted and the previous references remain authoritative.
