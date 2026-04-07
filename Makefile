PROTO_REPO      := https://github.com/dstl/SAPIENT-Proto-Files.git
PROTO_DIR       := build/proto-upstream
HARNESS_REPO    := https://github.com/dstl/BSI-Flex-335-v2-Test-Harness.git
HARNESS_DIR     := build/test-harness
STAGING         := build/proto-staging
FIXTURES_DIR    := $(HARNESS_DIR)/SapientServicesValidator.UnitTests

GO_PKG_V1       := sapient/pkg/sapientpb/v1
GO_PKG_V2       := sapient/pkg/sapientpb
PROTO_OPTS_SRC  := $(PROTO_DIR)/proto_options.proto

.PHONY: proto build test fuzz clean ci-up ci-down ci-test fixtures

proto: $(PROTO_DIR)
	@rm -rf $(STAGING) pkg/sapientpb
	@mkdir -p $(STAGING)/sapient_msg/bsi_flex_335_v1_0 \
	          $(STAGING)/sapient_msg/bsi_flex_335_v2_0 \
	          pkg/sapientpb pkg/sapientpb/v1
	@# proto_options.proto (shared, use v2 package so both can import it)
	@sed '/^package /a option go_package = "$(GO_PKG_V2)";' \
		$(PROTO_OPTS_SRC) > $(STAGING)/sapient_msg/proto_options.proto
	@# v2.0 protos
	@for f in $(PROTO_DIR)/bsi_flex_335_v2_0/*.proto; do \
		sed '/^package /a option go_package = "$(GO_PKG_V2)";' \
			"$$f" > "$(STAGING)/sapient_msg/bsi_flex_335_v2_0/$$(basename $$f)"; \
	done
	@# v1.0 protos
	@for f in $(PROTO_DIR)/bsi_flex_335_v1_0/*.proto; do \
		sed '/^package /a option go_package = "$(GO_PKG_V1)";' \
			"$$f" > "$(STAGING)/sapient_msg/bsi_flex_335_v1_0/$$(basename $$f)"; \
	done
	@echo "Patched v1.0 + v2.0 proto files"
	@# Generate v2.0
	protoc \
		--proto_path=$(STAGING) \
		--go_out=. \
		--go_opt=module=sapient \
		$(STAGING)/sapient_msg/proto_options.proto \
		$(STAGING)/sapient_msg/bsi_flex_335_v2_0/*.proto
	@# Generate v1.0
	protoc \
		--proto_path=$(STAGING) \
		--go_out=. \
		--go_opt=module=sapient \
		$(STAGING)/sapient_msg/bsi_flex_335_v1_0/*.proto
	@echo "Generated v2: $$(ls pkg/sapientpb/*.go | wc -l) files, v1: $$(ls pkg/sapientpb/v1/*.go | wc -l) files"

$(PROTO_DIR):
	@mkdir -p build
	git -c url.https://github.com/.insteadOf=ignore:// clone --depth 1 $(PROTO_REPO) $(PROTO_DIR)

$(HARNESS_DIR):
	@mkdir -p build
	git -c url.https://github.com/.insteadOf=ignore:// clone --depth 1 $(HARNESS_REPO) $(HARNESS_DIR)

fixtures: $(HARNESS_DIR)

build: proto
	go build ./...

test: build fixtures
	SAPIENT_FIXTURES_DIR=$(abspath $(FIXTURES_DIR)) go test -v -count=1 ./...

fuzz: build
	go test -run='^$$' -fuzz=FuzzRecv -fuzztime=10s ./pkg/sapient/
	go test -run='^$$' -fuzz=FuzzUnmarshalSapientMessage -fuzztime=10s ./pkg/sapient/
	go test -run='^$$' -fuzz=FuzzUnmarshalJSON -fuzztime=10s ./pkg/sapient/

ci-up:
	docker compose -f docker-compose.ci.yml up -d --build --wait

ci-down:
	docker compose -f docker-compose.ci.yml down

ci-test: proto fixtures ci-up test
	@echo "CI tests passed"

clean:
	rm -rf build/ pkg/sapientpb/
