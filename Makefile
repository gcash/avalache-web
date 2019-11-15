.DEFAULT_GOAL := help

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

##
## Build
##

ui: ## Build static UI
	@go-bindata -o ./api/bindata.go -prefix "static/" -pkg api -o bindata.go ui/...
	@shasum ./api/bindata.go