FROM alpine:3.21 AS ccc-mail-api-builder

RUN apk --no-cache update \
 && apk --no-cache upgrade \
 && apk --no-cache add ca-certificates go

COPY src/ccc-mail-api /usr/src/ccc-mail-api/

WORKDIR /usr/src/ccc-mail-api

RUN find .

RUN go test -race ./... \
 && cd cmd/ccc-mail-api \
 && CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -o ccc-mail-api


FROM alpine:3.21 AS procmail-builder

RUN apk --no-cache update \
 && apk --no-cache upgrade \
 && apk --no-cache add alpine-sdk

WORKDIR /usr/src

COPY src/procmail .

RUN tar xzvf procmail-3.22.tar.gz \
 && cd procmail-3.22 \
 && patch -p1 < ../procmail-3.22-consolidated_fixes-1.patch \
 && patch -p1 < ../procmail-3.22-getline.patch \
 && sed -i -e 's/^\(CFLAGS0 = -O\)/\1 -fpermissive/' Makefile \
 && make


FROM alpine:3.21

RUN apk --no-cache update \
 && apk --no-cache upgrade \
 && apk --no-cache add ca-certificates dovecot opendkim opendkim-utils postfix spamassassin spamassassin-client

COPY --from=ccc-mail-api-builder /usr/src/ccc-mail-api/cmd/ccc-mail-api/ccc-mail-api /usr/sbin

COPY --from=procmail-builder /usr/src/procmail-3.22/src/formail /usr/sbin
COPY --from=procmail-builder /usr/src/procmail-3.22/src/mailstat /usr/sbin
COPY --from=procmail-builder /usr/src/procmail-3.22/src/procmail /usr/sbin
COPY --from=procmail-builder /usr/src/procmail-3.22/src/setid /usr/sbin

ENV ENV=/root/.ashrc

COPY entrypoint /
COPY root /root

ENTRYPOINT ["/entrypoint"]
