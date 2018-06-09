from golang:1.10.3 as build
workdir /go/src/go.jonnrb.io/wifi_dash
add . .
run go get -u github.com/golang/dep/cmd/dep \
 && dep ensure \
 && CGO_ENABLED=0 GOOS=linux go build .

from gcr.io/distroless/base
copy --from=build /go/go.jonnrb.io/wifi_dash/wifi_dash /wifi_dash
add index.html /
add static/bootstrap.min.css /static/bootstrap.min.css
entrypoint ["/wifi_dash"]
