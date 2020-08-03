FROM gcr.io/jenkinsxio-labs-private/jxl-base:0.0.52

ENTRYPOINT ["jx-test"]

COPY ./build/linux/jx-test /usr/bin/jx-test