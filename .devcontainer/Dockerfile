ARG GO_VERSION=1.25.5

FROM golang:${GO_VERSION}-alpine

RUN apk add -q --update --progress --no-cache go gcc libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev linux-headers mesa-dev libxkbcommon-dev wayland-dev
RUN apk add -q --update --progress --no-cache nodejs npm
RUN go install golang.org/x/tools/gopls@v0.21.0
RUN go install github.com/go-delve/delve/cmd/dlv@v1.26.0
RUN go install fyne.io/tools/cmd/fyne@v1.7.0
RUN wget -O- -nv https://golangci-lint.run/install.sh | sh -s v2.7.2
