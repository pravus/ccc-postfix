FROM alpine:3 AS procmail-builder

RUN apk --no-cache update \
 && apk --no-cache upgrade \
 && apk --no-cache add alpine-sdk

WORKDIR /usr/src

COPY procmail .

RUN tar xzvf procmail-3.22.tar.gz \
 && cd procmail-3.22 \
 && patch -p1 < ../procmail-3.22-consolidated_fixes-1.patch \
 && patch -p1 < ../procmail-3.22-getline.patch \
 && make


FROM alpine:3

ENV ENV=/root/.ashrc

RUN apk --no-cache update \
 && apk --no-cache upgrade

RUN apk --no-cache add opendkim opendkim-utils postfix spamassassin spamassassin-client

COPY --from=procmail-builder /usr/src/procmail-3.22/src/formail /usr/sbin
COPY --from=procmail-builder /usr/src/procmail-3.22/src/mailstat /usr/sbin
COPY --from=procmail-builder /usr/src/procmail-3.22/src/procmail /usr/sbin
COPY --from=procmail-builder /usr/src/procmail-3.22/src/setid /usr/sbin

COPY entrypoint /
COPY root /root

ENTRYPOINT ["/entrypoint"]
