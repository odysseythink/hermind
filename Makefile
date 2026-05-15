.PHONY: build build-desktop-backend-macos build-desktop-backend-windows

build:
	go build -o bin/hermind ./cmd/hermind

build-desktop-backend-macos:
	go build -o desktop/resources/hermind-desktop-backend ./cmd/hermind

build-desktop-backend-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o desktop/resources/hermind-desktop-backend.exe ./cmd/hermind
