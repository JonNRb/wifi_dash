from golang:1.9 as build

workdir /go/src/github.com/jonnrb/wifi_dash
add . .

env CGO_ENABLED 0
env GOOS linux

run go-wrapper download
run go-wrapper install

from scratch
copy --from=build /go/bin/wifi_dash /wifi_dash

add ./index.html /index.html
add ./static/bootstrap.min.css /static/bootstrap.min.css

entrypoint ["/wifi_dash"]
