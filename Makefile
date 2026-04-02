PROTO_REPO   := https://github.com/dstl/SAPIENT-Proto-Files.git
PROTO_DIR    := build/proto-upstream
STAGING      := build/proto-staging
GO_PKG       := sapient/pkg/sapientpb
PROTO_V2_SRC := $(PROTO_DIR)/bsi_flex_335_v2_0
PROTO_OPTS_SRC := $(PROTO_DIR)/proto_options.proto

.PHONY: proto build test clean ci-up ci-down ci-test

proto: $(PROTO_DIR)
	@rm -rf $(STAGING) pkg/sapientpb
	@mkdir -p $(STAGING)/sapient_msg/bsi_flex_335_v2_0 pkg/sapientpb
	@sed '/^package /a option go_package = "$(GO_PKG)";' \
		$(PROTO_OPTS_SRC) > $(STAGING)/sapient_msg/proto_options.proto
	@for f in $(PROTO_V2_SRC)/*.proto; do \
		sed '/^package /a option go_package = "$(GO_PKG)";' \
			"$$f" > "$(STAGING)/sapient_msg/bsi_flex_335_v2_0/$$(basename $$f)"; \
	done
	@echo "Patched $$(ls $(STAGING)/sapient_msg/bsi_flex_335_v2_0/*.proto | wc -l) proto files"
	protoc \
		--proto_path=$(STAGING) \
		--go_out=. \
		--go_opt=module=sapient \
		$(STAGING)/sapient_msg/proto_options.proto \
		$(STAGING)/sapient_msg/bsi_flex_335_v2_0/*.proto
	@echo "Generated $$(ls pkg/sapientpb/*.go | wc -l) Go files in pkg/sapientpb/"

$(PROTO_DIR):
	@mkdir -p build
	git -c url.https://github.com/.insteadOf=ignore:// clone --depth 1 $(PROTO_REPO) $(PROTO_DIR)

build: proto
	go build ./...

test: build
	go test -v -count=1 ./...

clean:
	rm -rf build/ pkg/sapientpb/

ci-up:
	docker compose -f docker-compose.ci.yml up -d --build --wait

ci-down:
	docker compose -f docker-compose.ci.yml down

ci-test: proto ci-up test
	@echo "CI tests passed"
