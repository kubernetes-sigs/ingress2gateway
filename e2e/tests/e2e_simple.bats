#!/usr/bin/env bats

#### This test suite contains just the simple ingress test, and should not have any Provider specifics

load functions.bash

@test "Simple Ingress manifest test" {
    i2g simple/simple.yaml
    do_request
}