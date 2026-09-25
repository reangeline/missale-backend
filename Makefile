ENV ?= dev
TF   = terraform -chdir=terraform/environments/$(ENV)

.PHONY: test build plan deploy migrate run-local

test:
	go test -race ./...

build:
	mkdir -p build
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o build/bootstrap ./cmd/api
	cd build && rm -f api.zip && zip -q api.zip bootstrap

plan: build
	$(TF) init -input=false
	$(TF) plan -var-file=secrets.tfvars

deploy: test build
	$(TF) init -input=false
	$(TF) apply -var-file=secrets.tfvars

# Creates tables and the missale_api role mapped to the Lambda's IAM role.
migrate:
	go run ./cmd/migrate -host $$($(TF) output -raw dsql_endpoint) -lambda-role $$($(TF) output -raw lambda_role_arn) $(if $(FROM),-from $(FROM))
