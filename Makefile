build:
	mkdir -p var
	go build

install:build
	go install 

clean:
	go clean

test:conf_test.go parser/all_test.go
	@go test -v
	@go test -v ./parser

run:build
	@rm -f var/alser.lock
	./alser -v -debug -test -tail

up:
	git push origin master
	go get -u github.com/funkygao/alser/parser

doc:up
	@go doc github.com/funkygao/alser/parser

fmt:
	@gofmt -s -tabs=false -tabwidth=4 -w=true .

his:build
	./alser -c conf/alser.history.json

tail:build
	./alser -c conf/alser.json -tail
