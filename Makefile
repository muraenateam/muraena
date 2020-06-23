TARGET=muraena
PACKAGES=core log proxy session module module/crawler module/necrobrowser module/statichttp module/tracking

all: deps build

deps: godep golint gofmt gomegacheck updatedeps

build:
	export GO111MODULE=on
	@go build -o $(TARGET) .

racebuild:
	export GO111MODULE=on
	@go build -a -race -o $(TARGET) .

lint: gofmt
	@git add . && git commit -a -m "Linting :star2:"

clean:
	@rm -rf $(TARGET)
	@rm -rf build
	@dep prune

updatedeps:
	@dep ensure -update -v
	@dep prune
	@git add "Gopkg.*" "vendor"
	@git commit -m "Updated deps :star2:  (via Makefile)"

# tools
godep:
	@go get -u github.com/golang/dep/...

golint:
	@go get -u golang.org/x/lint/golint

gomegacheck:
	@go get honnef.co/go/tools/cmd/megacheck

gofmt:
	gofmt -s -w $(PACKAGES)
