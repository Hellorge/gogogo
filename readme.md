steps to run:

1. installed go

2. some more installs:
	- go get github.com/tdewolff/minify/v2
	- go get github.com/tdewolff/minify/v2/css
	- go get github.com/tdewolff/minify/v2/js
	- go get github.com/tdewolff/minify/v2/html
	- go get github.com/fsnotify/fsnotify
	- go get github.com/evanw/esbuild
	- and etc...

3. commands:
	cd into project root
	start server = go run ./cmd/main
	build assets = go run ./cmd/build
	check metrics = go run ./cmd/stats //broken as of now
