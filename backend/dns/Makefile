identdns: main.go
	GOOS=linux GOARCH=amd64 go build -o identdns .
.PHONY: ident tnedi
tnedi: identdns
	rsync -aP identdns tnedi:/usr/local/bin/identdns && ssh tnedi systemctl restart identdns
ident: identdns
	rsync -aP identdns ident:/usr/local/bin/identdns && ssh ident systemctl restart identdns
