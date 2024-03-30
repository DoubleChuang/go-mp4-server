# arm:	
# 	@# GOOS=linux GOARCH=arm64  go build .
# 	GOOS=linux GOARCH=arm GOARM=7 go build .
BIN = go-mp4-server
SRC = main.go
LINUX_BIN = $(BIN)_linux
MAC_BIN = $(BIN)_mac
PI_BIN = $(BIN)_pi
ARM64_BIN = $(BIN)_arm64
WIN_BIN = $(BIN)_win
MIPS_BIN = $(BIN)_mips
DEBUG_BIN = $(BIN)_debuge

BIN_DIR = bin/
LINUX_BIN_DIR = $(BIN_DIR)linux/
MAC_BIN_DIR = $(BIN_DIR)mac/
PI_BIN_DIR = $(BIN_DIR)pi/
ARM64_BIN_DIR = $(BIN_DIR)arm64/
WIN_BIN_DIR = $(BIN_DIR)win/
MIPS_BIN_DIR = $(BIN_DIR)mips/
DEBUG_BIN_DIR = $(BIN_DIR)debuge/


all:
	@make release
release:
	make mac
	make linux
	make win
	make pi
	make arm64
	make mips
mac:
	make $(MAC_BIN)
linux:
	make $(LINUX_BIN)
pi:
	make $(PI_BIN)
arm64:
	make $(ARM64_BIN)
win:
	make $(WIN_BIN)
mips:
	make $(MIPS_BIN)
debug:
	make $(DEBUG_BIN)
local:
	@go build -ldflags "-s -w" -o $(BIN_DIR)$(BIN) $(SRC)
docker:
	docker build -t go-mp4-server .

$(DEBUG_BIN):
	@go build -gcflags "-N -l" -o $(BIN) $(SRC)

$(MAC_BIN):
	@GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(MAC_BIN_DIR)$(BIN) $(SRC)
$(LINUX_BIN):
	@GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(LINUX_BIN_DIR)$(BIN) $(SRC)
$(PI_BIN):
	GOOS=linux GOARCH=arm go build -ldflags "-s -w" -o $(PI_BIN_DIR)$(BIN) $(SRC)
$(ARM64_BIN):
	GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o $(ARM64_BIN_DIR)$(BIN) $(SRC)
$(WIN_BIN):
	@GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(WIN_BIN_DIR)$(BIN).exe $(SRC)
$(MIPS_BIN):                                                                                                                                             
	@GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags "-s -w" -o $(MIPS_BIN_DIR)$(BIN) $(SRC)

clean:
	@rm -rf $(BIN_DIR)
.PHONY: all debug release win $(DEBUG_BIN) clean
