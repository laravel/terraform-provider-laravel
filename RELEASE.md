# Releasing the Laravel Cloud Terraform Provider

## Prerequisites

1. A GPG key for signing releases, registered on the Terraform Registry under
   the `laravel` namespace
2. Repository secrets configured:
   - `GPG_PRIVATE_KEY` — ASCII-armored GPG private key
   - `GPG_PASSPHRASE` — only required if the signing key carries a passphrase.
     The current release key (rsa4096 `B8F24FC7CA47EF85`,
     `terraform-provider@laravel.com`) does not, so this secret is unset and
     `${{ secrets.GPG_PASSPHRASE }}` resolves to the empty string, which
     `ghaction-import-gpg` treats as "no passphrase".

## First-time registry publication

These steps are needed once, before the first tag. Afterwards the Registry
tracks new GitHub releases on its own.

1. **The repository must be public.** The public Terraform Registry only
   indexes public GitHub repositories; while the repo is private it does not
   appear in the publish form's repository list at all.
2. Upload the **public** half of the signing key to the namespace, at
   Registry → Public Namespaces → `laravel` → GPG Keys:

   ```sh
   gpg --armor --export B8F24FC7CA47EF85 | pbcopy
   ```

3. Publish the provider at Registry → Public Namespaces → `laravel` → Publish
   Provider, selecting the `terraform-provider-laravel` repository. Existing
   releases with valid semver tags are imported; later ones are picked up
   automatically.

The provider's source address is `laravel/laravel` — the namespace is the
GitHub org and the provider name is the repository suffix. This is what
`main.go` serves as `registry.terraform.io/laravel/laravel` and what
`docs/index.md` tells users to put in `required_providers`.

## Creating a Release

1. Ensure all changes are merged to `main`
2. Update `CHANGELOG.md` with the new version's entries
3. Create and push a version tag:

```sh
git tag v1.0.0
git push origin v1.0.0
```

Pushing a `v*` tag triggers `.github/workflows/release.yml`, which will:

- Build binaries for all supported platforms via GoReleaser
- Create SHA256 checksums
- Sign the checksums with GPG
- Attach `terraform-registry-manifest.json` as `<project>_<version>_manifest.json`
- Publish a GitHub release

## Terraform Registry

Once the GitHub release is published, the Terraform Registry picks up the new
version automatically (after the provider has been registered once).

### Registry Requirements

- The repository must be public and named `terraform-provider-laravel`
- `terraform-registry-manifest.json` must be present at the repo root and
  attached to the release
- Release artifacts must be signed with the GPG key registered on the Registry

## Version Scheme

This provider follows [Semantic Versioning](https://semver.org/):

- **Major**: Breaking changes to resource schemas or behavior
- **Minor**: New resources, data sources, or backwards-compatible features
- **Patch**: Bug fixes and minor improvements
