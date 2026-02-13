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
			../../../../ingress2gateway print --input-file $$file --providers nginx --no-color > $$(dirname $$(dirname $$file))/output/$$(basename $$file) 2>&1; \
		done; \
	done

.PHONY: generate-clean-fixtures
generate-clean-fixtures: ## Generate fixtures without logs (clean YAML only)
	@echo "Generating clean fixtures..."
	@for dir in $(shell find fixtures -type d -name "input"); do \
		for file in $$(find $$dir -type f); do \
			echo "Processing $$file..."; \
			../../../../ingress2gateway print --input-file $$file --providers nginx --no-color | \
			awk '/^apiVersion:/ {found=1} found {print}' > $$(dirname $$(dirname $$file))/output/$$(basename $$file); \
		done; \
	done
