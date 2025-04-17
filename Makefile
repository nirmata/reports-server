.DEFAULT_GOAL := verify-codegen

ORG           ?= nirmata
PACKAGE       ?= github.com/kyverno/reports-server
GIT_SHA       := $(shell git rev-parse HEAD)
GOOS          := $(shell go env GOOS)
GOARCH        := $(shell go env GOARCH)

REGISTRY      ?= ghcr.io
REPO          ?= reports-server
IMAGE         := $(REGISTRY)/$(ORG)/$(REPO)

TOOLS_DIR        := .tools
OPENAPI_GEN      := $(shell go env GOPATH)/bin/openapi-gen
HELM             := $(TOOLS_DIR)/helm
HELM_DOCS_IMAGE  := jnorwood/helm-docs:v1.11.0

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

.PHONY: codegen-helm-docs
codegen-helm-docs:
	docker run \
	  -v $(PWD)/charts:/work \
	  -w /work \
	  $(HELM_DOCS_IMAGE) \
	  -s file

.PHONY: codegen-openapi
codegen-openapi:
	$(OPENAPI_GEN) \
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

.PHONY: codegen-install-manifest
codegen-install-manifest:
	$(HELM) template reports-server \
	  --namespace reports-server \
	  ./charts/reports-server \
	  --set apiServicesManagement.installApiServices.enabled=true \
	  --set image.tag=latest \
	  --set templating.enabled=true \
	| sed -e '/^#.*/d' > config/install.yaml

.PHONY: codegen-install-manifest-etcd
codegen-install-manifest-etcd:
	$(HELM) template reports-server \
	  --namespace reports-server \
	  ./charts/reports-server \
	  --set apiServicesManagement.installApiServices.enabled=true \
	  --set image.tag=latest \
	  --set config.etcd.enabled=true \
	  --set postgresql.enabled=false \
	  --set templating.enabled=true \
	| sed -e '/^#.*/d' > config/install-etcd.yaml

.PHONY: codegen
codegen: codegen-helm-docs codegen-openapi codegen-install-manifest codegen-install-manifest-etcd

.PHONY: verify-codegen
verify-codegen: codegen
	@echo "Checking codegen is up to date…"
	git diff --exit-code
