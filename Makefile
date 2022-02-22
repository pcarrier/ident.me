identssh: *.go
	GOOS=linux GOARCH=amd64 go build -o identssh .
.PHONY: ident tnedi
tnedi: identssh
	rsync -aP identssh tnedi: && ssh tnedi 'doas bash -c "install identssh /usr/local/bin/identssh; systemctl restart identssh"'
ident: identssh
	rsync -aP identssh ident: && ssh ident 'doas bash -c "install identssh /usr/local/bin/identssh; systemctl restart identssh"'
