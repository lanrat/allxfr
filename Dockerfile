# build stage
FROM golang:alpine AS build-env
RUN apk update && apk add --no-cache make git

WORKDIR /go/app/
COPY . .
RUN make

# final stage
FROM alpine

COPY --from=build-env /go/app/allxfr /bin/allxfr

USER nobody

ENTRYPOINT ["allxfr"]