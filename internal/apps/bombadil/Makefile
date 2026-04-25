multiplier ?= 1
BOMBADIL_MULTIPLIER ?= $(multiplier)
BOMBADIL_FLAGS := --headless --exit-on-violation --chrome-grant-permissions ''

.PHONY: bombadil bombadil-build bombadil-build-react bombadil-build-preact bombadil-build-solid
.PHONY: bombadil-react bombadil-preact bombadil-solid validate-multiplier

bombadil: validate-multiplier
	@$(MAKE) --no-print-directory -j3 bombadil-react bombadil-preact bombadil-solid

bombadil-build:
	@$(MAKE) --no-print-directory -j3 bombadil-build-react bombadil-build-preact bombadil-build-solid

bombadil-build-react:
	@pnpm build:react

bombadil-build-preact:
	@pnpm build:preact

bombadil-build-solid:
	@pnpm build:solid

validate-multiplier:
	@case "$(BOMBADIL_MULTIPLIER)" in \
		''|*[!0-9]*|0) \
			echo "BOMBADIL_MULTIPLIER must be a positive integer, e.g. make bombadil multiplier=2"; \
			exit 2; \
			;; \
	esac

define BOMBADIL_RUN_VARIANT
	@server_log="$${TMPDIR:-/tmp}/vorma-bombadil-$(1).log"; \
	baseline_limit="$$(($(BOMBADIL_MULTIPLIER) * 30))s"; \
	nested_limit="$$(($(BOMBADIL_MULTIPLIER) * 15))s"; \
	counter_limit="$$(($(BOMBADIL_MULTIPLIER) * 10))s"; \
	PORT=$(2) VORMA_BOMBADIL_VARIANT=$(1) ./.dist.$(1)/.vorma/main > "$$server_log" 2>&1 & \
	server_pid=$$!; \
	cleanup() { kill $$server_pid >/dev/null 2>&1 || true; wait $$server_pid >/dev/null 2>&1 || true; }; \
	trap cleanup EXIT INT TERM; \
	ready=0; \
	for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 47 48 49 50; do \
		if curl -fsS http://127.0.0.1:$(2)/ >/dev/null 2>&1; then ready=1; break; fi; \
		if ! kill -0 $$server_pid >/dev/null 2>&1; then break; fi; \
		sleep 0.1; \
	done; \
	if [ "$$ready" != "1" ]; then \
		cat "$$server_log"; \
		exit 1; \
	fi; \
	status=0; \
	pnpm exec bombadil test http://127.0.0.1:$(2) ./specs/vorma.spec.ts $(BOMBADIL_FLAGS) --time-limit "$$baseline_limit" --output-path ./.bombadil/$(1) && \
		pnpm exec bombadil test http://127.0.0.1:$(2)/nested/alpha/details ./specs/vorma.spec.ts $(BOMBADIL_FLAGS) --time-limit "$$nested_limit" --output-path ./.bombadil/$(1)-nested && \
		pnpm exec bombadil test 'http://127.0.0.1:$(2)/counter?n=0' ./specs/vorma.spec.ts $(BOMBADIL_FLAGS) --time-limit "$$counter_limit" --output-path ./.bombadil/$(1)-counter || status=$$?; \
	cleanup; \
	trap - EXIT INT TERM; \
	exit $$status
endef

bombadil-react: bombadil-build-react
	$(call BOMBADIL_RUN_VARIANT,react,18080)

bombadil-preact: bombadil-build-preact
	$(call BOMBADIL_RUN_VARIANT,preact,18081)

bombadil-solid: bombadil-build-solid
	$(call BOMBADIL_RUN_VARIANT,solid,18082)
