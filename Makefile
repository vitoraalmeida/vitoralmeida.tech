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

build:
	go run ./cmd/sitegen build \
		--content ./content \
		--templates ./templates \
		--static ./static \
		--output ./dist

verify:
	test -f dist/index.html
	test -f dist/404.html
	test -f dist/about.html
	test -f dist/blog.html
	test -f dist/portfolio.html
	test -f dist/styles/global.css
	test -f dist/fonts/inconsolata-latin.woff2
	test -f dist/robots.txt
