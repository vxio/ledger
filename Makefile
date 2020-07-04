.PHONY: compile
compile:
	protoc api/v1/*.proto \
		--gogo_out=\
Mgogoproto/gogo.proto=github.com/gogo/protobuf/proto,plugins=grpc:. \
		--proto_path=\
$$(go list -f '{{ .Dir }}' -m github.com/gogo/protobuf) \
		--proto_path=.

CONFIG_PATH=${HOME}/.ledger/

.PHONY: init
init:
	mkdir -p ${CONFIG_PATH}
	cp test/model.conf $(CONFIG_PATH)/model.conf
	cp test/policy.csv $(CONFIG_PATH)/policy.csv

.PHONY: gencert
gencert:
	cfssl gencert \
		-initca test/ca-csr.json | cfssljson -bare ca

	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=server \
		test/server-csr.json | cfssljson -bare server
# END: begin

# START: client
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=client \
		-cn="root" \
		test/client-csr.json | cfssljson -bare root-client
# END: client

# START: client
	cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=client \
		-cn="nobody" \
		test/client-csr.json | cfssljson -bare nobody-client
# END: client

	mv *.pem *.csr ${CONFIG_PATH}

.PHONY: test
test:
	go test -race ./...

