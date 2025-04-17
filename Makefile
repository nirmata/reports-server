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
	@echo "Create gopath shim…" >&2
	mkdir -p $(GOPATH_SHIM)

$(PACKAGE_SHIM): $(GOPATH_SHIM)
	@echo "Create package shim…" >&2
	mkdir -p $(GOPATH_SHIM)/src/github.com/kyverno && \
	  ln -sf ${PWD} $(PACKAGE_SHIM)

.PHONY: codegen-openapi
codegen-openapi: $(PACKAGE_SHIM) $(OPENAPI_GEN)
	@echo "Generate openapi…" >&2
	$(OPENAPI_GEN) \
	  -i k8s.io/apimachinery/pkg/api/resource \
	  -i k8s.io/apimachinery/pkg/apis/meta/v1 \
	  -i k8s.io/apimachinery/pkg/runtime \
	  -i k8s.io/api/core/v1 \
	  -i sigs.k8s.io/wg-policy-prototypes/policy-report/pkg/api/wgpolicyk8s.io/v1alpha2 \
	  -i github.com/kyverno/kyverno/api/reports/v1 \
	  -i github.com/kyverno/kyverno/api/policyreport/v1alpha2 \
	  -p ./pkg/api/generated/openapi \
	  -O zz_generated.openapi \
	  -h ./.hack/boilerplate.go.txt

.PHONY: codegen-helm-docs
codegen-helm-docs:
	@echo "Generate helm docs…" >&2
	docker run -v ${PWD}/charts:/work -w /work jnorwood/helm-docs:v1.11.0 -s file

.PHONY: codegen-install-manifest
codegen-install-manifest:
	@echo "Generate latest install manifest…" >&2
	$(HELM) template reports-server --namespace reports-server ./charts/reports-server \
	  --set apiServicesManagement.installApiServices.enabled=true \
	  --set image.tag=latest \
	  --set templating.enabled=true \
	  | sed -e '/^#.*/d' \
	  > ./config/install.yaml

.PHONY: codegen
codegen: codegen-helm-docs codegen-openapi codegen-install-manifest

.PHONY: verify-codegen
verify-codegen: codegen
	@echo "Checking codegen is up to date…" >&2
	git diff --quiet || (echo "Run 'make codegen && git add .' locally, then re‑commit." >&2; false)
