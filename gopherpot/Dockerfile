#  Derleme aşaması 
FROM golang:1.22-bookworm AS builder
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gopherpot .

#  Çalışma aşaması (minimal, distroless benzeri) 
FROM gcr.io/distroless/static-debian12
COPY --from=builder /gopherpot /gopherpot
EXPOSE 2222 8080
ENTRYPOINT ["/gopherpot"]
