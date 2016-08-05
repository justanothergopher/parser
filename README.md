# simple rest server
accepts post on /api/v1/parse as a json { "message":"xyz" }
returns json with mentions, emoticons and url:title pairs
see parse.go for more details

instrumentation/status: 
  /debug/vars

selftests:
  /selftest
  /bulktest

External dependencies: golang.org/x/net/html (use go get golang.org/x/net/html)

