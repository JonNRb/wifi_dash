from golang:1.9 as build

workdir /go/src/github.com/jonnrb/wifi_dash
add . .
run CGO_ENABLED=0 GOOS=linux go-wrapper download \
 && CGO_ENABLED=0 GOOS=linux go-wrapper install

from scratch
copy --from=build /go/bin/wifi_dash /wifi_dash
add ./index.html ./static/bootstrap.min.css /
entrypoint ["/wifi_dash"]
