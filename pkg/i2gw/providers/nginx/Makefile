# Generate fixtures.
# This target assumes that the 'ingress2gateway' binary has already been built
# and is located at the root of the project.
# Captures both logs (stderr) and YAML output (stdout) to the fixture file.
.PHONY: generate-fixtures
generate-fixtures: ## Generate fixture output files from input manifests
	@echo "Generating fixtures..."
	@for dir in $(shell find fixtures -type d -name "input"); do \
		for file in $$(find $$dir -type f); do \
			echo "Processing $$file..."; \
			../../../../ingress2gateway print --input-file $$file --providers nginx > $$(dirname $$(dirname $$file))/output/$$(basename $$file) 2>&1; \
		done; \
	done
