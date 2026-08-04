.PHONY: test check clean build verify

test:
	go test ./...

check:
	go run ./cmd/sitegen check \
		--content ./content \
		--templates ./templates \
		--static ./static

clean:
	rm -rf dist .tmp

build: clean
	mkdir -p .tmp
	go run ./cmd/sitegen build \
		--content ./content \
		--templates ./templates \
		--static ./static \
		--output .tmp/site
	mv .tmp/site dist

verify:
	test -f dist/index.html
	test -f dist/404.html
	test -f dist/about.html
	test -f dist/blog.html
	test -f dist/portfolio.html
	test -f dist/styles/global.css
	test -f dist/robots.txt
