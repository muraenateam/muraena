BUILD     ?= build
TARGET    ?= muraena
PACKAGES  ?= core log session module module/crawler module/necrobrowser module/statichttp module/tracking module/watchdog
GO        ?= go

all: build

# This will be triggered before any command, or when just calling $ make
pre:
	mkdir -p $(BUILD)
    # GO111MODULE is required only when inside GOPATH
	env GO111MODULE=on go get -d ./

build: pre
	$(GO) build -o $(BUILD)/$(TARGET) .

build_with_race_detector: pre
	$(GO) build -race -o $(BUILD)/$(TARGET) .

buildall: pre
	env GO111MODULE=on GOOS=darwin GOARCH=amd64 go build -o $(BUILD)/macos/$(TARGET) .
	env GO111MODULE=on GOOS=linux GOARCH=amd64 go build -o $(BUILD)/linux/$(TARGET) .
	env GO111MODULE=on GOOS=windows GOARCH=amd64 go build -o  $(BUILD)/windows/$(TARGET).exe .

update:
	go get -u
	go mod vendor
	go mod tidy
	@git commit go.mod go.sum -m "Bump dependencies 📈"

lint: fmt
	@git add . && git commit -a -m "Code linting :star2:"

fmt:
	gofmt -s -w $(PACKAGES)

.PHONY: all build build_with_race_detector lint fmt
