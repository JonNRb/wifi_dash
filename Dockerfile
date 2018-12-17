from quay.io/jonnrb/go as build
add . /src
run cd /src && GOOS=linux CGO_ENABLED=0 go get -v .

from gcr.io/distroless/base
copy --from=build /go/bin/wifi_dash /wifi_dash
add index.html /
add static /static
entrypoint ["/wifi_dash"]
