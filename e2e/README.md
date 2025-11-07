# Ingress2Gateway e2e tests

This folder contains the e2e tests for ingress2gateway. Implementors willing to 
add new features or migrate MUST add tests to the subfolder of the provider, to 
guarantee that the desired migration is working as expected.

The tests are written using [BATS](https://bats-core.readthedocs.io/en/stable/index.html)
to guarantee that the desired user behavior happens as they were really operating 
a cluster and avoiding to write Go programs that may not reflect the desired 
behavior of the test.

To execute the tests, you should run from the root directory of the project the 
command `make e2e`.
