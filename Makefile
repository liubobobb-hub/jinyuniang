.PHONY: run test tidy fmt demo

ADDR ?= 127.0.0.1:8787

run:
	go run ./cmd/whale-live

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	gofmt -w cmd internal

demo:
	WHALE_HOST=http://$(ADDR) ./scripts/send_danmaku.sh "小鲸鱼，帮我讲讲这段代码"
