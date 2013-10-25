test:
	@go test -v
	cd parser; go test -v

run:
	@rm -f var/alser.lock
	./alser -v -debug -test -tail -pprof var/cpu.prof

fmt:
	@gofmt -s -tabs=false -tabwidth=4 -w=true .

prof:
	@go tool pprof alser var/cpu.prof

his:
	@rm -f var/*
	./alser -c conf/alser.history.json

tail:
	while true; do \
		rm -f var/*; \
		./alser -c conf/alser.json -tail; \
		sleep 3; \
	done
