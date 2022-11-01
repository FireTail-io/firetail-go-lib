# Minimal AWS Lambda example

This example was taken from the [serverless/examples](https://github.com/serverless/examples) repo's [aws-golang-simple-http-endpoint](https://github.com/serverless/examples/tree/master/aws-golang-simple-http-endpoint) example, and modified to work with the [serverless offline](https://www.serverless.com/plugins/serverless-offline) golang runner which has [some quirks](https://github.com/dherault/serverless-offline/issues/1358).



## Development Environment

Install [serverless](https://www.serverless.com/), [serverless offline](https://www.serverless.com/plugins/serverless-offline) and [golang](https://go.dev/), then:

```bash
git clone git@github.com:FireTail-io/firetail-go-lib.git
cd firetail-go-lib/examples/minimal-lambda
npm install
sls offline
```

Curl it!

```bash
curl 'localhost:3000/hello'
```

You should get the following response:

```json
{"message":"Go Serverless v1.0! Your function executed successfully!"}
```



## Cloud Deployment

To deploy this project to AWS, a makefile is included. You'll first need to build the binaries:

```bash
make build
```

You can then deploy them to AWS using the serverless framework and clean your local environment of any temporary files:

```bash
make deploy
make clean
```

You can also remove them from AWS when you're done:

```bash
make remove
```
