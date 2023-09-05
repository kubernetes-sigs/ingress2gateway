# Release Process

The Ingress2Gateway Project is CLI project that helps with translating Ingress and provider related resources to [Gateway API](https://github.com/kubernetes-sigs/gateway-api) resources.

The release process is as follows:

## For a MAJOR or MINOR release

<!-- TODO(liorlieberman) -->

### For a RC release

1. Open a PR with changes of the version references in the codebase.
2. Include necessary changelog updates to CHANGELOG.md in this PR.
3. Merge the PR
4. Tag the release using the commit on main where the PR merged. This can be done using the git CLI `git tag -sa $VERSION`.
5. Run `git push origin $VERSION`, this will trigger a github workflow that will create the release.
6. Verify the [releases page](https://github.com/kubernetes-sigs/ingress2gateway/releases) to ensure that the release meets the expectations.
7. All [OWNERS](OWNERS) must LGTM this release
<!-- TODO(liorlieberman) is this needed? -->
8.  An announcement email is sent to `dev@kubernetes.io` with the subject `[ANNOUNCE] kubernetes-template-project $VERSION is released`
