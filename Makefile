INSTALL_DIR  := $(HOME)/.local/bin
BINARY       := gl1tch-mud
PIPELINE_SRC := pipelines
PIPELINE_DST := $(HOME)/.config/glitch/pipelines

.PHONY: build install install-pipelines clean

build:
	go build -o $(BINARY) .

install: build install-pipelines
	install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	rm $(BINARY)

install-pipelines:
	@mkdir -p $(PIPELINE_DST)
	@for f in $(PIPELINE_SRC)/*.pipeline.yaml; do \
		name=$$(basename "$$f"); \
		dest=$(PIPELINE_DST)/$$name; \
		if [ ! -f "$$dest" ]; then \
			cp "$$f" "$$dest"; \
			echo "installed: $$name"; \
		else \
			echo "skip (exists): $$name"; \
		fi \
	done

clean:
	rm -f $(BINARY)
