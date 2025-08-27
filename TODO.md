# TODO

## add greppable output mode

status, package category etc. should all be on one line. no emojis

## inconsistent formatting

last updated information should be formatted consistently in output across different statuses

example problem:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
❌ github.com/grpc-ecosystem/go-grpc-prometheus - Repository is archived
   Last updated: 21 days ago
❌ github.com/aws/aws-sdk-go - Repository is archived
❌ github.com/golang/snappy - Repository is archived
   Last updated: 4 days ago

❓ UNKNOWN STATUS PACKAGES (9 found):
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
❓ go.uber.org/zap - Active Uber-maintained Go package (trusted)
❓ golang.org/x/crypto - Active Go extended package, last updated 1 days ago
```
