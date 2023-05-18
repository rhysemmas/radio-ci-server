.PHONY: install build clean

install: build
	chmod 0777 bin/radio-ci-server
	sudo cp bin/radio-ci-server /usr/local/bin/

	chmod 0644 radio-ci-server.service
	sudo cp radio-ci-server.service /lib/systemd/system/
	@echo ""
	@echo "--------------------------------------------------"
	@echo "!!! Remember to update your token in the systemd unit !!!"
	@echo ""
	@echo "Then enable your service with: "
	@echo "sudo systemctl daemon-reload"
	@echo "sudo systemctl enable radio-ci-server.service"
	@echo "--------------------------------------------------"
	@echo ""

build: $(shell find . -iname '*.go')
	go build -o bin/radio-ci-server main.go

clean:
	rm -rf bin
