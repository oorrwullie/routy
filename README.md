#routy
Routy is a speedy little Edge Proxy with deny list IP blocking and SSL integration with Let's Encrypt.

## Installation
#### Ubuntu
You should be able to simply run `sudo make install`.

This will install the binary to `/usr/local/bin` and configure a deamon for you.

#### Everyone Else
```
go build . -o routy
sudo mv routy /usr/local/bin
sudo chgrp www-data /usr/local/bin/routy
sudo chmod g+x /usr/local/bin/routy
```
If your user does not have permission to run servers on ports 80 and 443:
```
sudo @iptables -A INPUT -p tcp --dport 80 -j ACCEPT
sudo @iptables -A INPUT -p tcp --dport 443 -j ACCEPT
```
NOTE: If you already have wildcard SSL certificates from Let's Encrypt, copy them into either `/var/routy/certs` or `$HOME/routy/certs`.

## Configuration And Logs
All configuration and log files are found in either `/var/routy` or `$HOME/routy`.
* access.log:           The log file for all incoming requests
* certs:                Directory containing the Let's Encrypt certificates
* cfg.json:             basic configuration file for Routy
* denyList.json:        list of IP addresses to deny access to routes
* events.log:           The log file for all server events and information
* subdomainRoutes.json: subdomain/target combos to proxy routes to

### cfg.json
The cfg.json file contains the configuration for the base hostname. A typical configuration looks like this:
```
{
    "hostname": "example.com"
}
```

### Subdomain Routes
A typical subdomainRoutes.json file will look like this:
```
[
    {
        subdomain: "mysubdomain",
        target: "http://localhost:8080"
    },
    {
        subdomain: "example",
        target: "http://example.example"
    }
]
```

### Deny List
A typical denyList.json file will look like this:
```
[
    "100.15.126.231",
    "100.19.145.164",
    "101.100.139.201",
    "27.33.100.62"
]
```

## License
Licensed under the [MIT License](http://github.com/oorrwullie/routy/blob/master/LICENSE).
