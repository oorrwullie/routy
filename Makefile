OS := $(shell uname -s)

ifeq ($(OS),Linux)
        DISTRO := $(shell cat /etc/os-release | grep ^NAME | cut -d'=' -f2 | sed 's/\"//gI')
endif

install:
ifeq ($(OS),Linux)
		ifeq ($(DISTRO)),Ubuntu)
				@go build -o routy
				@iptables -A INPUT -p tcp --dport 80 -j ACCEPT
				@iptables -A INPUT -p tcp --dport 443 -j ACCEPT
				@mv routy /usr/local/bin
				@chgrp wheel /usr/local/bin/routy
				@chmod g+x /usr/local/bin/routy
				@mkdir /var/routy
				@chgrp wheel /var/routy
				@if [ -d /etc/systemd/system ]; then cp scripts/routy.service /etc/systemd/system ; fi
				@systemctl enable routy.service
				@systemctl start routy.service
		else
				@echo $(DISTRO) "is not supported in make install."
		endif
else
		@echo $(OS) "is not supported in make install."
endif