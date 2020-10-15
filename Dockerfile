ARG BASE_IMAGE=opensuse/leap

FROM golang:1.14 as build
ADD . /eirini-secscanner
WORKDIR /eirini-secscanner
RUN CGO_ENABLED=0 go build -o eirini-secscanner
RUN chmod +x eirini-secscanner

FROM $BASE_IMAGE
COPY --from=build /eirini-secscanner/eirini-secscanner /bin/
ENTRYPOINT ["/bin/eirini-secscanner"]
