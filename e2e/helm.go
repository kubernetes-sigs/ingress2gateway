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
	"context"
	"fmt"
	"os"
	"time"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
)

func installChart(
	ctx context.Context,
	log logger,
	settings *cli.EnvSettings,
	repoURL string,
	releaseName string,
	chartName string,
	version string,
	namespace string,
	createNamespace bool,
	values map[string]interface{},
) error {
	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return fmt.Errorf("initializing helm config: %w", err)
	}

	// Check if release already exists.
	status := action.NewStatus(cfg)
	if _, err := status.Run(releaseName); err == nil {
		log.Logf("Release %q already exists, skipping installation", releaseName)
		return nil
	}

	install := action.NewInstall(cfg)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.CreateNamespace = createNamespace
	install.WaitStrategy = kube.StatusWatcherStrategy
	install.Timeout = 5 * time.Minute
	install.RepoURL = repoURL
	install.Version = version

	cp, err := locateChart(ctx, log, install, chartName, settings)
	if err != nil {
		return fmt.Errorf("locating chart: %w", err)
	}

	chartRequested, err := loader.Load(cp)
	if err != nil {
		return fmt.Errorf("loading chart: %w", err)
	}

	_, err = install.RunWithContext(ctx, chartRequested, values)
	if err != nil {
		return fmt.Errorf("running install: %w", err)
	}

	return nil
}

func uninstallChart(ctx context.Context, settings *cli.EnvSettings, releaseName, namespace string) error {
	// Helm's Uninstall action doesn't support context so we can only check before starting.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled before uninstall: %w", err)
	}

	cfg := new(action.Configuration)

	if err := cfg.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER")); err != nil {
		return fmt.Errorf("Initializing helm config: %w", err)
	}

	uninstall := action.NewUninstall(cfg)
	uninstall.WaitStrategy = kube.StatusWatcherStrategy
	// The default deletion propagation mode is "background", which may cause some resources to be
	// left behind for a while, which in turn may lead to "stuck" namespace deletions after
	// removing a release.
	// Relevant issue: https://github.com/helm/helm/issues/31651
	uninstall.DeletionPropagation = "foreground"
	uninstall.Timeout = 5 * time.Minute

	_, err := uninstall.Run(releaseName)
	if err != nil {
		return fmt.Errorf("Uninstalling %s: %w", releaseName, err)
	}

	return nil
}

func locateChart(
	ctx context.Context,
	log logger,
	install *action.Install,
	chartName string,
	settings *cli.EnvSettings,
) (string, error) {
	// Helm masks the underlying HTTP errors and status codes so we can't easily distinguish
	// transient errors (e.g. 503) from permanent errors (e.g. 404). Rather than relying on
	// fragile string parsing, we treat all errors as transient failures. This isn't a big
	// problem since the whole retry process is fairly short.
	return retryWithData(ctx, log, defaultRetryConfig(),
		func(attempt, maxAttempts int, err error) string {
			return fmt.Sprintf("Locating chart (attempt %d/%d): %v", attempt, maxAttempts, err)
		},
		func() (string, error) {
			return install.ChartPathOptions.LocateChart(chartName, settings)
		},
	)
}
