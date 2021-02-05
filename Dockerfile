FROM ghcr.io/jenkins-x/jx-boot:latest

ENTRYPOINT ["jx-test"]

COPY ./build/linux/jx-test /usr/bin/jx-test
