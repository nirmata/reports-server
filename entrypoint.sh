# yaml-language-server: $schema=https://json.schemastore.org/github-workflow.json

name: Publish images FIPS

permissions: {}

on:
  push:
    branches:
    - "*"

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  publish-images:
    runs-on: ubuntu-latest
    permissions:
      packages: write
      id-token: write
    outputs:
      reports-server-digest: ${{ steps.publish-reports-server.outputs.digest }}
    steps:
    - name: Checkout
      uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1
    - name: Setup caches
      uses: ./.github/actions/setup-caches
      timeout-minutes: 5
      continue-on-error: true
      with:
        build-cache-key: publish-images
    - name: Setup build env
      uses: ./.github/actions/setup-build-env
      timeout-minutes: 30
    - name: Install Cosign
      uses: sigstore/cosign-installer@e1523de7571e31dbe865fd2e80c5c7c23ae71eb4 # v3.4.0
    - name: Publish reports server
      id: publish-reports-server
      uses: ./.github/actions/publish-image
      with:
        makefile-target: ko-publish-reports-server-fips
        registry: ghcr.io
        registry-username: ${{ github.actor }}
        registry-password: ${{ secrets.GITHUB_TOKEN }}
        repository: reports-server
        version: ${{ github.ref_name }}
        sign-image: true
        sbom-name: reports-server-fips
        sbom-repository: ghcr.io/${{ github.repository_owner }}/reports-server-fips/sbom
        signature-repository: ghcr.io/${{ github.repository_owner }}/reports-server-fips/signatures
        main-path: .
  generate-reports-server-provenance:
    needs: publish-images
    permissions:
      id-token: write # To sign the provenance.
      packages: write # To upload assets to release.
      actions: read # To read the workflow path.
    # NOTE: The container generator workflow is not officially released as GA.
    uses: slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@v2.0.0
    with:
      image: ghcr.io/${{ github.repository_owner }}/reports-server
      digest: "${{ needs.publish-images.outputs.reports-server-digest }}"
      registry-username: ${{ github.actor }}
    secrets:
      registry-password: ${{ secrets.GITHUB_TOKEN }}