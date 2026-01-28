/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"sync"
)

var globalResourceManager = &resourceManager{
	resources: make(map[string]*resourceState),
}

// Manages shared resources used by tests. It allows safe reuse of resources which have expensive
// setup and/or teardown by multiple concurrent tests.
type resourceManager struct {
	// The methods on this type are designed to return immediately: Any long-running operation
	// should run asynchronously. The mutex is used only for thread-safe access to the internal
	// state and is NOT designed to remain locked while a long-running resource operation is
	// executing.
	mu        sync.Mutex
	resources map[string]*resourceState
}

// Returns a shared resource identified by key.
//
// If the resource does not exist, install is called asynchronously to create it. The returned
// Resource allows callers to wait for installation to complete and to trigger cleanup. Subsequent
// calls with the same key return immediately without calling install again.
//
// Each caller MUST call the cleanup() method on the returned Resource to ensure resource release
// takes place.
func (rm *resourceManager) acquire(key string, install installFunc) resource {
	for {
		rm.mu.Lock()
		state, exists := rm.resources[key]
		if exists && state.cleaningUp != nil {
			// Resource is being cleaned up - wait and retry.
			cleaningUp := state.cleaningUp
			rm.mu.Unlock()
			<-cleaningUp
			continue
		}

		if !exists {
			state = &resourceState{
				ready: make(chan struct{}),
				count: 0,
			}
			rm.resources[key] = state

			// Run installation asynchronously.
			go func() {
				defer close(state.ready)
				cleanup, err := install()
				if err != nil {
					state.err = err
					return
				}
				state.cleanup = cleanup
			}()
		}
		state.count++
		rm.mu.Unlock()

		done := make(chan struct{})
		var once sync.Once // Protect against multiple cleanups by same caller

		return resource{
			name: key,
			cleanup: func() <-chan struct{} {
				once.Do(func() {
					go func() {
						<-rm.release(key)
						close(done)
					}()
				})
				return done
			},
			wait: func() error {
				<-state.ready
				return state.err
			},
		}
	}
}

// Decrements the reference count for a resource and triggers cleanup when the count reaches zero.
// Returns a channel that is closed when cleanup completes.
func (rm *resourceManager) release(key string) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		rm.mu.Lock()
		state, ok := rm.resources[key]
		if !ok {
			rm.mu.Unlock()
			return
		}

		state.count--
		if state.count <= 0 {
			// Mark the resource as cleaning up before releasing the lock. This prevents new
			// Acquire calls from using a resource that is being cleaned up.
			state.cleaningUp = make(chan struct{})
			rm.mu.Unlock()

			// Wait for installation to complete before running cleanup.
			<-state.ready
			if state.cleanup != nil {
				state.cleanup()
			}

			// Remove the resource from the map and signal cleanup is done.
			rm.mu.Lock()
			delete(rm.resources, key)
			close(state.cleaningUp)
			rm.mu.Unlock()
		} else {
			rm.mu.Unlock()
		}
	}()

	return done
}

// Represents a resource managed by the resourceManager.
type resource struct {
	// A name for this resource. Useful for error messages.
	name string
	// Releases the resource's underlying resources.
	cleanup func() <-chan struct{}
	// Blocks until the resource is installed and ready for use. If there was an error during the
	// installation, the error is returned.
	wait func() error
}

// A synchronous install function which returns a synchronous cleanup function or an installation
// error.
type installFunc func() (cleanupFunc, error)

// A function which contains logic for cleaning up a resource.
type cleanupFunc func()

// Tracks a shared resource's state.
type resourceState struct {
	cleanup    cleanupFunc
	ready      chan struct{} // Closed when installation completes
	cleaningUp chan struct{} // Closed when cleanup completes
	err        error         // An installation error
	count      int           // Reference count
}
