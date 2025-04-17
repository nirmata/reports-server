.DEFAULT_GOAL := build

##########
# CONFIG #
##########

ORG                                ?= nirmata
PACKAGE                            ?= github.com/kyverno/reports-server
GIT_SHA                            := $(shell git rev-parse HEAD)
GOOS                               ?= $(shell go env GOOS)
GOARCH                             ?= $(shell go env GOARCH)
REGISTRY                           ?= ghcr.io
REPO                               ?= reports-server
REPO_REPORTS_SERVER	?= 	$(REGISTRY)/$(ORG)/$(REPO)

# FORCE Go to use your vendor/ directory if present
export GOFLAGS                  := -mod=vendor

#########
# TOOLS #
#########

TOOLS_DIR                          := $(PWD)/.tools
REGISTER_GEN                       := $(TOOLS_DIR)/register-gen
OPENAPI_GEN                        := $(TOOLS_DIR)/openapi-gen
CODE_GEN_VERSION                   := v0.29.8
KIND                               := $(TOOLS_DIR)/kind
KIND_VERSION                       := v0.23.0
KO                                 := $(TOOLS_DIR)/ko
KO_VERSION                         := v0.14.1
HELM                               := $(TOOLS_DIR)/helm
HELM_VERSION                       := v3.10.1
TOOLS                              := $(REGISTER_GEN) $(OPENAPI_GEN) $(KIND) $(KO) $(HELM)

ifeq ($(GOOS), darwin)
SED                                := gsed
else
SED                                := sed
endif

###########
# VENDOR  #
###########

.PHONY: vendor
vendor: go.mod go.sum
	@echo "Vendoring all dependencies…" >&2
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
codegen-openapi: vendor $(PACKAGE_SHIM) $(OPENAPI_GEN)
	@echo Generate openapi... >&2
	GOFLAGS=-mod=vendor $(OPENAPI_GEN) \
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
codegen-helm-docs:
	@echo Generate helm docs... >&2
	docker run -v ${PWD}/charts:/work -w /work \
		jnorwood/helm-docs:v1.11.0 -s file

.PHONY: codegen-install-manifest
codegen-install-manifest: $(HELM)
	@echo Generate latest install manifest... >&2
	$(HELM) template reports-server --namespace reports-server ./charts/reports-server/ \
		--set apiServicesManagement.installApiServices.enabled=true \
		--set image.tag=latest \
		--set templating.enabled=true \
 		| $(SED) -e '/^#.*/d' \
		> ./config/install.yaml

.PHONY: codegen-install-manifest-etcd
codegen-install-manifest-etcd: $(HELM)
	@echo Generate latest install manifest (etcd)… >&2
	$(HELM) template reports-server --namespace reports-server ./charts/reports-server/ \
		--set apiServicesManagement.installApiServices.enabled=true \
		--set image.tag=latest \
		--set config.etcd.enabled=true \
		--set postgresql.enabled=false \
		--set templating.enabled=true \
 		| $(SED) -e '/^#.*/d' \
		> ./config/install-etcd.yaml

.PHONY: codegen
codegen: codegen-helm-docs codegen-openapi codegen-install-manifest codegen-install-manifest-etcd

.PHONY: verify-codegen
verify-codegen: codegen
	@echo Checking codegen is up to date... >&2
	@git diff --quiet --exit-code -- .
	@echo 'If this test fails, run "make codegen", commit, and push.' >&2
