# Releasing the Laravel Cloud Terraform Provider

## Prerequisites

1. A GPG key for signing releases, registered on the Terraform Registry under
   the `laravel` namespace
2. Repository secrets configured (and the release token, see
   [Release token](#release-token)):
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

Releases are cut by [release-please](https://github.com/googleapis/release-please)
(`.github/workflows/release-please.yml`). There is no changelog to edit and no
tag to push.

1. Merge pull requests into `main` with a
   [conventional-commit](https://www.conventionalcommits.org/) title. The
   `PR Title` check enforces this, and the squash-merged title is what
   release-please reads:

   | Title                                   | Changelog section | Bump  |
   | --------------------------------------- | ----------------- | ----- |
   | `feat(environment): add foo`            | Features          | minor |
   | `fix(import): stop false diffs`         | Bug Fixes         | patch |
   | `perf: …`, `revert: …`, `deps: …`       | own section       | patch |
   | `docs:`, `refactor:`, `test:`, `ci:`, `build:`, `chore:` | hidden | none |
   | `feat!: …` or a `BREAKING CHANGE:` footer | flagged as breaking | major |

   Write the title for provider users: it becomes the changelog line. Put
   upgrade notes a user must act on in a `BREAKING CHANGE:` footer in the
   squash commit body, which release-please copies into the changelog.

2. release-please keeps a `chore(main): release X.Y.Z` pull request open and
   updates it on every push to `main`, with the next version and the
   `CHANGELOG.md` entry. Review the entry there. release-please regenerates
   both the PR body (which becomes the release notes) and `CHANGELOG.md` on
   that branch on every push to `main`, so hand edits only survive if they
   are made right before merging the release PR.

3. Merge the release PR. release-please then creates the `vX.Y.Z` tag and a
   **draft** GitHub release with the notes. The tag push runs
   `.github/workflows/release.yml`, which:

   - Validates the tag is semver and not lower than the latest tag
   - Builds binaries for all supported platforms via GoReleaser
   - Creates SHA256 checksums and signs them with GPG
   - Attaches `terraform-registry-manifest.json` as `<project>_<version>_manifest.json`
   - Uploads everything into the draft and then publishes it

   The release stays a draft until every asset is uploaded, because the
   Registry ingests a version as soon as its release is published.

To force a specific version, add a `Release-As: 2.0.0` footer to a commit on
`main`.

### Release token

GitHub does not start workflows for events caused by the built-in
`GITHUB_TOKEN`, so release-please authenticates as a GitHub App. Otherwise
the release PR would never run the required CI checks, and the tag would
never start `release.yml`.

- The App needs **Contents: read & write** and **Pull requests: read & write**
  on this repository.
- Store its App ID as the repository **variable** `RELEASE_APP_ID` and its
  private key as the repository **secret** `RELEASE_APP_PRIVATE_KEY`.

### Manual fallback

Pushing a `v*` tag by hand still runs `release.yml`. With no release-please
draft to reuse, GoReleaser creates the release itself and generates notes
from the commit log. Add the matching `CHANGELOG.md` entry and bump
`.release-please-manifest.json` in a follow-up PR, or release-please will
propose the same version again.

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
