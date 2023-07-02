all:
OS := $(shell uname -s)

ifeq ($(OS),Linux)
        DISTRO := $(shell cat /etc/os-release | grep ^NAME | cut -d'=' -f2 | sed 's/\"//gI')
endif

install:
ifeq ($(OS),Linux)
ifeq ($(DISTRO),Ubuntu)
	@go build -o routy
	@iptables -A INPUT -p tcp --dport 80 -j ACCEPT
	@iptables -A INPUT -p tcp --dport 443 -j ACCEPT
	@mv routy /usr/local/bin
	@chgrp www-data /usr/local/bin/routy
	@chmod g+x /usr/local/bin/routy
	@mkdir /var/routy
	@chgrp www-data /var/routy
	@if [ -d /etc/systemd/system ]; then cp scripts/routy.service /etc/systemd/system ; fi
	@systemctl enable routy.service
else
	@echo $(DISTRO) "is not supported in make install."
endif
else
	@echo $(OS) "is not supported in make install."
endif

clean:
ifeq ($(OS),Linux)
ifeq ($(DISTRO),Ubuntu)
	@rm -rf /usr/local/bin/routy
	@rm -rf /var/routy
	@systemctl stop routy.service
	@systemctl disable routy.service
	@rm /etc/systemd/system/routy.service
endif
endif
