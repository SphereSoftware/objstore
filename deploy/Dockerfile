FROM gliderlabs/alpine

COPY out/objstore /objstore/bin/objstore

ENV APP_DEBUG_LEVEL=0
ENV APP_CLUSTER_TAGNAME=default

VOLUME /objstore/data
WORKDIR /objstore/data

EXPOSE 10999 10080

CMD ["/objstore/bin/objstore"]
