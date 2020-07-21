FROM golang:1.14-alpine AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/ledger ./cmd/ledger

FROM scratch
COPY --from=build /go/bin/ledger /bin/ledger

ENTRYPOINT ["/bin/ledger"]
# END: probes



