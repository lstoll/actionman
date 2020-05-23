FROM golang:1.14 AS build

WORKDIR /actionman

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go install ./...

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /go/bin/actionman /usr/bin/

ENTRYPOINT ["/usr/bin/actionman"]
