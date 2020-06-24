TARGET    ?= muraena
PACKAGES  ?= core log proxy session module module/crawler module/necrobrowser module/statichttp module/tracking
GO        ?= go

all: build

# This will be triggered before any command, or when just calling $ make
pre:
	mkdir -p ./build/
    # GO111MODULE is required only when inside GOPATH
	env GO111MODULE=on go get -d ./

build: pre
	$(RM) -f ./build/$(TARGET)
	$(GO) build -o ./build/$(TARGET) .

build_with_race_detector: pre
	$(GO) build -race -o $(TARGET) .


buildall: pre
	$(RM) -r ./build
	mkdir -p ./build/windows
	mkdir -p ./build/linux
	mkdir -p ./build/macos
	env GO111MODULE=on GOOS=darwin GOARCH=amd64 go build -o ./build/macos/$(TARGET) .
	env GO111MODULE=on GOOS=linux GOARCH=amd64 go build -o ./build/linux/$(TARGET) .
	env GO111MODULE=on GOOS=windows GOARCH=amd64 go build -o  ./build/windows/$(TARGET).exe .

lint: fmt
	@git add . && git commit -a -m "Linting :star2:"

fmt:
	gofmt -s -w $(PACKAGES)



.PHONY: all build build_with_race_detector lint fmt