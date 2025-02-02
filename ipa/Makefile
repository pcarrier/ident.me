all: dist/index.html dist/style.css

dist/index.html: index.haml
	haml index.haml > dist/index.html

dist/style.css: style.scss
	scss -t compressed style.scss > dist/style.css

clean:
	rm dist/index.html dist/style.css

deploy: all
	xmit ipa.tnedi.me
