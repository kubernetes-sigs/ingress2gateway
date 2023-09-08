# Release Process

## Overview

The Ingress2Gateway Project is CLI project that helps with translating Ingress and provider related resources to [Gateway API](https://github.com/kubernetes-sigs/gateway-api) resources.

## Releasing a new version

### Writing a Changelog

To simplify release notes generation, we recommend using the [Kubernetes release
notes generator](https://github.com/kubernetes/release/blob/master/cmd/release-notes):

```
go install k8s.io/release/cmd/release-notes@latest
export GITHUB_TOKEN=your_token_here
release-notes --start-sha EXAMPLE_COMMIT --end-sha EXAMPLE_COMMIT --branch main --repo ingress2gateway --org kubernetes-sigs
```

This output will likely need to be reorganized and cleaned up a bit, but it
provides a good starting point. Once you're satisfied with the changelog, create
a PR. This must go through the regular PR review process and get merged into the
`main` branch. Approval of the PR indicates community consensus for a new
release.

### Patch a release

<!-- TODO(liorlieberman) -->

### Release a MAJOR or MINOR release

<!-- TODO(liorlieberman) -->

### Release a RC release

1. Open a PR with changes of the version references in the codebase.
2. Include necessary changelog updates to CHANGELOG.md in this PR.
4. Merge the PR
5. Tag the release using the commit on main where the PR merged. This can be done using the git CLI `git tag -sa $VERSION`.
6. Run `git push origin $VERSION`, this will trigger a github workflow that will create the release.
7. Verify the [releases page](https://github.com/kubernetes-sigs/ingress2gateway/releases) to ensure that the release meets the expectations.
<!-- TODO(liorlieberman) is this needed? -->
8.  An announcement email is sent to `dev@kubernetes.io` with the subject `[ANNOUNCE] kubernetes-template-project $VERSION is released`
