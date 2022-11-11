protoc --go_out=. ./proto/*.proto
go build .
cp tcp.com ./builds/sample1/ 
cp tcp.com ./builds/sample2/ 