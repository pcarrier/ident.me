identtelnet: main.go
	GOOS=linux GOARCH=amd64 go build -o identtelnet .
.PHONY: ident tnedi
tnedi: identtelnet
	rsync -aP identtelnet tnedi: && ssh tnedi 'doas bash -c "install identtelnet /usr/local/bin/identtelnet; systemctl restart identtelnet"'
ident: identtelnet
	rsync -aP identtelnet ident:/usr/local/bin/identtelnet && ssh ident systemctl restart identtelnet
