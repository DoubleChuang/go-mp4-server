arm:	
	@# GOOS=linux GOARCH=arm64  go build .
	GOOS=linux GOARCH=arm GOARM=7 go build .
