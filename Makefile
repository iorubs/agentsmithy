MODEL ?= qwen3:8b
MCPSMITHY_PORT ?= 8090

.PHONY: ollama-check ollama-up ollama-down

ollama-check:
	@command -v ollama >/dev/null 2>&1 || { echo "ollama not installed. Run: brew install ollama"; exit 1; }
	@if ollama list >/dev/null 2>&1; then \
		echo "ollama is running at http://localhost:11434"; \
	else \
		echo "ollama is installed but not running. Run: make ollama-up"; \
		exit 1; \
	fi

ollama-up:
	@command -v ollama >/dev/null 2>&1 || { echo "ollama not installed. Run: brew install ollama"; exit 1; }
	@brew services start ollama
	@echo "waiting for ollama to come up..."
	@until ollama list >/dev/null 2>&1; do sleep 1; done
	@ollama list | grep -q "$(MODEL)" || ollama pull $(MODEL)
	@echo "ollama up, serving $(MODEL) at http://localhost:11434"

ollama-down:
	@brew services stop ollama

.PHONY: mcpsmithy-up mcpsmithy-down

mcpsmithy-up:
	@docker run --rm -d --name mcpsmithy -p $(MCPSMITHY_PORT):$(MCPSMITHY_PORT) \
		-v "$(PWD)":/project:ro -w /project \
		smithylabs/mcpsmithy:latest serve --transport http --addr :$(MCPSMITHY_PORT)
	@echo "mcpsmithy serving .mcpsmithy.yaml at http://localhost:$(MCPSMITHY_PORT)"

mcpsmithy-down:
	@docker rm -f mcpsmithy
