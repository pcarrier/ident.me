identhttp: main.go
	GOOS=linux GOARCH=amd64 go build -o identhttp .
.PHONY: ident tnedi
tnedi: identhttp
	rsync -aP identhttp tnedi:/usr/local/bin/identhttp && ssh tnedi systemctl restart identhttp
ident: identhttp
	rsync -aP identhttp ident:/usr/local/bin/identhttp && ssh ident systemctl restart identhttp
