all: dist/index.html dist/style.css

dist/index.html: index.haml
	bundle exec haml index.haml > dist/index.html

dist/style.css: style.scss
	bundle exec sassc -t compressed style.scss dist/style.css

clean:
	rm dist/index.html dist/style.css

deploy: all
	xmit api.ident.me
