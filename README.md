# go-check-smtp

:construction: :construction: :construction:

go-check-smtp is a simple drop-in replacement for check\_smtp in nagios-plugins distribution written in Go.

## Motivation

I think check\_smtp with STARTTLS seems to be broken because check\_smtp does not trigger warning or critical even though SMTP server returns 4xx/5xx.

My SMTP server speaks [HAProxy's proxy protocol](http://www.haproxy.org/download/1.5/doc/proxy-protocol.txt) to receive connections from AWS's ELB so I also need check\_smtp with proxy protocol support.

## Installation

Download from [releases](https://github.com/nabeken/go-check-smtp/releases).

Or

```sh
go get -u github.com/nabeken/go-check-smtp/check_smtp
```

## Usage

```sh
check_smtp -S \
  -F localhost \
  -H 127.0.0.1 \
  -p 10025 \
  -w 2.0 \
  -c 1.0 \
  -C 'MAIL FROM:<sender@example.com>' \
  -R '250 2.1.0 Ok' \
  -C 'RCPT TO:<recipient@example.com>' \
  -R '250 2.1.5 Ok'
```
