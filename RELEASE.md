# Release Process

## Overview

The Ingress2Gateway Project is a CLI project that helps translate Ingress and provider related resources to [Gateway API](https://github.com/kubernetes-sigs/gateway-api) resources.

## Releasing a new version

### Writing a Changelog

To simplify release notes generation, we recommend using the [Kubernetes release
notes generator](https://github.com/kubernetes/release/blob/master/cmd/release-notes):

```
go install k8s.io/release/cmd/release-notes@latest
export GITHUB_TOKEN=your_token_here
release-notes --start-sha EXAMPLE_COMMIT --end-sha EXAMPLE_COMMIT --branch main --repo ingress2gateway --org kubernetes-sigs --required-author=""
```

This output will likely need to be reorganized and cleaned up a bit, but it
provides a good starting point. Once you're satisfied with the changelog, create
a PR. This must go through the regular PR review process and get merged into the
`main` branch. Approval of the PR indicates community consensus for a new
release.

### Patch a release

1. Create a new branch in your fork named something like `<githubuser>/release-x.x.x`. Use the new branch
  in the upcoming steps.
1. Use `git` to cherry-pick all relevant PRs into your branch.
1. Update the version references in the codebase with the new semver tag.
1. Create a pull request of the `<githubuser>/release-x.x.x` branch into the `release-x.x` branch upstream
  (which should already exist since this is a patch release). Add a hold on this PR waiting for at least
  one maintainer/codeowner to provide a `lgtm`.
1. Create a tag using the `HEAD` of the `release-x.x` branch. This can be done using the `git` CLI or
  Github's [release][release] page.

### Release a MAJOR or MINOR release

1. Cut a `release-major.minor` branch that we can tag things in as needed.
1. Check out the `release-major.minor` release branch locally.
1. Update the version references in the codebase with the new semver tag.
1. Verify the changelog is up to date with the desired changes.
1. Create a tag using the `HEAD` of the `release-x.x` branch. This can be done using `git tag -sa $VERSION` CLI or
  Github's [release][release] page.
1. Run `git push origin $VERSION`, this will trigger a github workflow that will create the release.
1. Verify the [releases page](https://github.com/kubernetes-sigs/ingress2gateway/releases) to ensure that the release meets the expectations.
1. Optional: Send an annoncement email to `kubernetes-sig-network@googlegroups.com` with the subject `[ANNOUNCE] ingress2gateway $VERSION is released`


### Release a RC release

1. Open a PR with changes of the version references in the codebase.
1. Include necessary changelog updates to CHANGELOG.md in this PR.
1. Merge the PR
1. Tag the release using the commit on main where the PR merged. This can be done using the git CLI `git tag -sa $VERSION`.
1. Run `git push origin $VERSION`, this will trigger a github workflow that will create the release.
1. Verify the [releases page](https://github.com/kubernetes-sigs/ingress2gateway/releases) to ensure that the release meets the expectations.
1. Optional: Send an annoncement email to `kubernetes-sig-network@googlegroups.com` with the subject `[ANNOUNCE] ingress2gateway $VERSION is released`

