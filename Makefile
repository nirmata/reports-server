.DEFAULT_GOAL := verify-codegen

##########
# CONFIG #
##########

ORG             ?= nirmata
PACKAGE         ?= github.com/kyverno/reports-server
GIT_SHA         := $(shell git rev-parse HEAD)
GOOS            ?= $(shell go env GOOS)
GOARCH          ?= $(shell go env GOARCH)

#########
# TOOLS #
#########
TOOLS_DIR       := $(PWD)/.tools
HELM            := $(TOOLS_DIR)/helm
OPENAPI_GEN     := $(TOOLS_DIR)/openapi-gen

###########
# VENDOR  #
###########

.PHONY: vendor
vendor:
	@echo "↳ downloading modules and updating vendor/…" >&2
	go mod tidy
	go mod vendor

###########
# CODEGEN #
###########


GOPATH_SHIM     := ${PWD}/.gopath
PACKAGE_SHIM    := $(GOPATH_SHIM)/src/$(PACKAGE)

$(GOPATH_SHIM):
	@echo Create gopath shim... >&2
	@mkdir -p $(GOPATH_SHIM)

.INTERMEDIATE: $(PACKAGE_SHIM)
$(PACKAGE_SHIM): $(GOPATH_SHIM)
	@echo Create package shim... >&2
	@mkdir -p $(GOPATH_SHIM)/src/github.com/kyverno && ln -s -f ${PWD} $(PACKAGE_SHIM)

.PHONY: codegen-openapi
codegen-openapi: $(PACKAGE_SHIM) $(OPENAPI_GEN) ## Generate openapi
	@echo Generate openapi... >&2
	@$(OPENAPI_GEN) \
		-i k8s.io/apimachinery/pkg/api/resource \
		-i k8s.io/apimachinery/pkg/apis/meta/v1 \
		-i k8s.io/apimachinery/pkg/version \
		-i k8s.io/apimachinery/pkg/runtime \
		-i k8s.io/apimachinery/pkg/types \
		-i k8s.io/api/core/v1 \
		-i sigs.k8s.io/wg-policy-prototypes/policy-report/pkg/api/wgpolicyk8s.io/v1alpha2 \
		-i github.com/kyverno/kyverno/api/reports/v1 \
		-i github.com/kyverno/kyverno/api/policyreport/v1alpha2 \
		-p ./pkg/api/generated/openapi \
		-O zz_generated.openapi \
		-h ./.hack/boilerplate.go.txt

.PHONY: codegen-helm-docs
codegen-helm-docs: ## Generate helm docs
	@echo Generate helm docs... >&2
	@docker run -v ${PWD}/charts:/work -w /work jnorwood/helm-docs:v1.11.0 -s file

.PHONY: codegen-install-manifest
codegen-install-manifest: $(HELM) ## Create install manifest
	@echo Generate latest install manifest... >&2
	@$(HELM) template reports-server --namespace reports-server ./charts/reports-server/ \
		--set apiServicesManagement.installApiServices.enabled=true \
		--set image.tag=latest \
		--set templating.enabled=true \
 		| $(SED) -e '/^#.*/d' \
		> ./config/install.yaml

codegen-install-manifest-etcd: $(HELM) ## Create install manifest without postgres
	@echo Generate latest install manifest... >&2
	@$(HELM) template reports-server --namespace reports-server ./charts/reports-server/ \
		--set apiServicesManagement.installApiServices.enabled=true \
		--set image.tag=latest \
		--set config.etcd.enabled=true \
		--set postgresql.enabled=false \
		--set templating.enabled=true \
 		| $(SED) -e '/^#.*/d' \
		> ./config/install-etcd.yaml

.PHONY: codegen
codegen: ## Rebuild all generated code and docs
codegen: codegen-helm-docs
codegen: codegen-openapi
codegen: codegen-install-manifest
codegen: codegen-install-manifest-etcd

.PHONY: verify-codegen
verify-codegen: codegen ## Verify all generated code and docs are up to date
	@echo Checking codegen is up to date... >&2
	@git --no-pager diff -- .
	@echo 'If this test fails, it is because the git diff is non-empty after running "make codegen".' >&2
	@echo 'To correct this, locally run "make codegen", commit the changes, and re-run tests.' >&2
	@git diff --quiet --exit-code -- .
