from quay.io/jonnrb/go as build
add . /src
run cd /src && GOOS=linux CGO_ENABLED=0 go get .

from gcr.io/distroless/static
copy --from=build /go/bin/wifi_dash /wifi_dash
add index.html /
add static /static
entrypoint ["/wifi_dash"]
