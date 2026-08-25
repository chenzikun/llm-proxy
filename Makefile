.PHONY: help

pull:
	@docker pull harbor-energy.auteltech.cn/dev/llm-proxy:1.0

push:
	@docker push harbor-energy.auteltech.cn/dev/llm-proxy:1.0