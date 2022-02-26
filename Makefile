identdns: *.go
	GOOS=linux GOARCH=amd64 go build -o identdns .
.PHONY: ident tnedi
tnedi: identdns
	rsync -aP identdns tnedi: && ssh tnedi 'doas bash -c "install identdns /usr/local/bin/identdns; systemctl restart identdns"'
ident: identdns
	rsync -aP identdns ident: && ssh ident 'doas bash -c "install identdns /usr/local/bin/identdns; systemctl restart identdns"'
