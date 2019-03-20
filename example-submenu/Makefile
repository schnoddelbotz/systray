
TARGET = example-submenu
SOURCE = main.go

PLATFORMS = darwin/amd64 windows/amd64 linux/amd64

all: $(TARGET)

$(TARGET): $(SOURCE)
	go build

crosscompile: custom-xgo
	go get -v github.com/karalabe/xgo
	xgo -image custom-xgo -targets "$(PLATFORMS)" .

custom-xgo:
	docker build -t custom-xgo .

clean:
	rm -f $(TARGET)
	rm -f $(TARGET)-{darwin,windows,linux}*
