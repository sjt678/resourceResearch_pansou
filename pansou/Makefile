# PanSou 构建与开发辅助

.PHONY: build run test newplugin

build:
	go build ./...

run:
	go run .

test:
	go test ./...

# 生成新插件脚手架: make newplugin NAME=foo
newplugin:
	bash scripts/newplugin.sh $(NAME)
