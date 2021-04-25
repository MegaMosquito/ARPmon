# A tool to monitor the local area network (LAN) "*/24" network using ARP.

DOCKERHUB_ID:=ibmosquito
NAME:="arpmon"
VERSION:="1.0.0"

# REST service location on the host
PORT:=1234
URL_BASE:=/v1/arpmon

# The number of goroutines to spawn (use 4 to keep RPi 3 load under 1.0)
GOROUTINES:=8

# If you set CIDR in your environment it will be used. Otherwise this will be.
DEFAULT_CIDR:=$(shell sh -c "ip route | grep default | head -1 | sed 's/ proto dhcp / /' | cut -d' ' -f3 | cut -d'.' -f1-3").0/24

# If you set IPV4 in your environment it will be used. Otherwise this will be.
DEFAULT_IPV4:=$(word 7, $(shell sh -c "ip route | grep default | sed 's/dhcp src //'"))

# If you set MAC in your environment it will be used. Otherwise this will be.
DEFAULT_MAC:=$(word 2,$(shell sh -c "ifconfig $(word 5, $(shell sh -c "ip route | grep default | sed 's/dhcp src //'")) | sed 'N;s/\n/ /;s/.*ether //;s/ .*//;'"))

default: build run

chkvars:
	@echo "Variable settings that will be used:"
	@echo "        CIDR: \"$(if $(CIDR),$(CIDR),$(DEFAULT_CIDR))\""
	@echo "        IPV4: \"$(if $(IPV4),$(IPV4),$(DEFAULT_IPV4))\""
	@echo "         MAC: \"$(if $(MAC),$(MAC),$(DEFAULT_MAC))\""
	@echo "        PORT: \"$(PORT)\""
	@echo "    URL_BASE: \"$(URL_BASE)\""
	@echo "  GOROUTINES: \"$(GOROUTINES)\""

build:
	docker build -t $(DOCKERHUB_ID)/$(NAME):$(VERSION) .

dev: stop build
	docker run -it -v `pwd`:/outside \
          --name ${NAME} \
          --privileged \
          --net host \
          -e CIDR=$(if $(CIDR),$(CIDR),$(DEFAULT_CIDR)) \
          -e IPV4=$(if $(IPV4),$(IPV4),$(DEFAULT_IPV4)) \
          -e MAC=$(if $(MAC),$(MAC),$(DEFAULT_MAC)) \
          -e PORT=$(PORT) \
          -e URL_BASE=$(URL_BASE) \
          -e GOROUTINES=$(GOROUTINES) \
          $(DOCKERHUB_ID)/$(NAME):$(VERSION) /bin/bash

run: stop
	docker run -d \
          --name ${NAME} \
          --restart unless-stopped \
          --privileged \
          --net host \
          -e CIDR=$(if $(CIDR),$(CIDR),$(DEFAULT_CIDR)) \
          -e IPV4=$(if $(IPV4),$(IPV4),$(DEFAULT_IPV4)) \
          -e MAC=$(if $(MAC),$(MAC),$(DEFAULT_MAC)) \
          -e PORT=$(PORT) \
          -e URL_BASE=$(URL_BASE) \
          -e GOROUTINES=$(GOROUTINES) \
          $(DOCKERHUB_ID)/$(NAME):$(VERSION)

macs:
	@curl -sS http://127.0.0.1:$(PORT)$(URL_BASE)/macs

csv:
	@curl -sS http://127.0.0.1:$(PORT)$(URL_BASE)/csv

check:
	@curl -sS http://127.0.0.1:$(PORT)$(URL_BASE)/json | jq .

push:
	docker push $(DOCKERHUB_ID)/$(NAME):$(VERSION) 

stop:
	@docker rm -f ${NAME} >/dev/null 2>&1 || :

clean:
	@docker rmi -f $(DOCKERHUB_ID)/$(NAME):$(VERSION) >/dev/null 2>&1 || :

.PHONY: chkvar build dev run push macs csv check stop clean
