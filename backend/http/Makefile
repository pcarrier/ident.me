identhttp: main.go
	GOOS=linux GOARCH=amd64 go build -o identhttp .
.PHONY: ident tnedi
tnedi: identhttp
	rsync -aP identhttp tnedi: && ssh tnedi 'doas bash -c "install identhttp /usr/local/bin/identhttp; systemctl restart identhttp"'
ident: identhttp
	rsync -aP identhttp ident: && ssh ident 'doas bash -c "install identhttp /usr/local/bin/identhttp; systemctl restart identhttp"'
