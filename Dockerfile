FROM golang:1.24-alpine
ENV CGO_ENABLED=0

WORKDIR /usr/src/app
COPY . .
RUN go build -o tfcbd *.go

FROM alpine
COPY --from=0 /usr/src/app/tfcbd /

CMD [ "./tfcbd" ]
