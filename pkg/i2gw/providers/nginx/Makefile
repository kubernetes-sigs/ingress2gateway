# Generate fixtures.
# This target assumes that the 'ingress2gateway' binary has already been built
# and is located at the root of the project.
.PHONY: generate-fixtures
generate-fixtures: ## Generate fixtures with notifications (for debugging)
	@echo "Generating fixtures..."
	@for dir in $(shell find fixtures -type d -name "input"); do \
		for file in $$(find $$dir -type f); do \
			echo "Processing $$file..."; \
			../../../../ingress2gateway print --input-file $$file --providers nginx > $$(dirname $$(dirname $$file))/output/$$(basename $$file); \
		done; \
	done

.PHONY: generate-clean-fixtures  
generate-clean-fixtures: ## Generate fixtures without notifications (clean YAML only)
	@echo "Generating clean fixtures..."
	@for dir in $(shell find fixtures -type d -name "input"); do \
		for file in $$(find $$dir -type f); do \
			echo "Processing $$file..."; \
			../../../../ingress2gateway print --input-file $$file --providers nginx | \
			awk '/^apiVersion:/ {found=1} found {print}' > $$(dirname $$(dirname $$file))/output/$$(basename $$file); \
		done; \
	done