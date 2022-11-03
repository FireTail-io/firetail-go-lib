build:
	cp app-spec.yaml hello/app-spec.yaml
	env GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -o bin/hello hello/main.go
	env GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -o bin/world world/main.go

.PHONY: clean
clean:
	rm -rf ./bin
	rm hello/app-spec.yaml

.PHONY: deploy
deploy: clean build
	sls deploy --verbose -c serverless.prod.yml

.PHONY: remove
remove:
	sls remove --verbose -c serverless.prod.yml