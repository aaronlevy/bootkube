FROM alpine

COPY bootkube /bootkube
COPY checkpoint /checkpoint
COPY install.sh /checkpoint-installer.sh
COPY checkpoint.json /checkpoint.json
