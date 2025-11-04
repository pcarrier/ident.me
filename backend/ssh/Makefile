identssh: main.go
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o identssh .
.PHONY: ident tnedi
tnedi: identssh
	rsync -aP identssh tnedi:/usr/local/bin/identssh && ssh tnedi systemctl restart identssh
ident: identssh
	rsync -aP identssh ident:/usr/local/bin/identssh && ssh ident systemctl restart identssh
