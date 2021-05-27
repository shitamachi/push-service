.PHONY: build proto clean deps

# generate the binary file for the program runtime platform
build:
	CGO_ENABLED=1 GO111MODULE=on go build -o ./bin/target/push-service main.go

# generate the binary file for linux platform
build-linux:
	 GO111MODULE=on GOOS=linux GOARCH=amd64 go build -o ./bin/linux/push-service main.go

proto:
	@./script/proto.sh

clean:
	rm -rf bin/
	rm -rf output/

deps:
	GOPRIVATE=code.byted.org GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor
	#git add vendor
	#go get -u github.com/golang/protobuf/protoc-gen-go

update-vendor:
	GO111MODULE=on go get -u
	#go get -u github.com/golang/protobuf/protoc-gen-go

cover:
	go test -coverprofile=cover.out -gcflags=-l ./pkg/... && go tool cover -html=cover.out && rm cover.out

cilint:
	golangci-lint run

golint:
	@go list ./... | grep -v -e vendor | xargs golint -set_exit_status -min_confidence 0


