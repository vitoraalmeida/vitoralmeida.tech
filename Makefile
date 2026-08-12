.PHONY: test check clean build verify

test:
	go test ./...
	python3 -m unittest scripts/test_deploy_static.py

check:
	go run ./cmd/sitegen check \
		--content ./content \
		--templates ./templates \
		--static ./static

clean:
	rm -rf dist

NOINDEX ?= 0

build:
	./scripts/optimize_assets.sh
	go run ./cmd/sitegen build \
		--content ./content \
		--templates ./templates \
		--static ./static \
		--output ./dist \
		$(if $(filter 1,$(NOINDEX)),--noindex,)

verify:
	test -f dist/index.html
	test -f dist/404.html
	test -f dist/about.html
	test -f dist/blog.html
	test -f dist/portfolio.html
	test -f dist/styles/global.css
	test -f dist/fonts/ibm-plex-mono.woff2
	test -f dist/fonts/space-mono-regular.woff2
	test -f dist/fonts/space-mono-bold.woff2
	test -f dist/robots.txt
	test -f dist/sitemap.xml
	test -f dist/feed.xml
	test -f dist/og-image.png
