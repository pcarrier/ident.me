identtelnet: main.go
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o identtelnet .
.PHONY: ident tnedi
tnedi: identtelnet
	rsync -aP identtelnet tnedi:/usr/local/bin/identtelnet && ssh tnedi systemctl restart identtelnet
ident: identtelnet
	rsync -aP identtelnet ident:/usr/local/bin/identtelnet && ssh ident systemctl restart identtelnet
