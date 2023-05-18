.PHONY: install build clean

install: build
	chmod 0777 bin/radio-ci-server
	mv bin/radio-ci-server /usr/local/bin/

	chmod 0644 radio-ci-server.service
	mv radio-ci-server.service /lib/systemd/system/
	echo "Remember to update your token in the systemd unit!"
	echo "Then enable your service with: "
	echo "systemctl daemon-reload"
	echo "systemctl enable radio-ci-server.service"

build: $(shell find . -iname '*.go')
	go build -o bin/radio-ci-server main.go

clean:
	rm -rf bin
