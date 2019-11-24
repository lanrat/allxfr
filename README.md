# ALL XFR

### AXFR all the things!

<img src="https://user-images.githubusercontent.com/164192/69487828-764bc280-0e15-11ea-8f67-41dd27ac32d8.jpg" width="400" height="300">

This tool performs a [zone transfer (AXFR)](https://en.wikipedia.org/wiki/DNS_zone_transfer) against the root zone servers to obtain the [root zone file](https://www.iana.org/domains/root/files). And then attempts a zone transfer for every IP for every nameserver for a given zone.

Most zones do not allow zone transfers, however a few do. Sometimes only on a single IP for a given nameserver and not the others. This tool will try them all and save every successful transfer.

This tool works best on an IPv4/IPv6 dual stack internet connection.

## Example

```
./allxfr -out zones
2019/11/23 17:26:23 ROOT xfr size: 22091 records (envelopes 77)
2019/11/23 17:26:25 sj. server.nordu.net. (193.10.252.19) xfr size: 15 records (envelopes 1)
2019/11/23 17:26:25 sj. server.nordu.net. (2001:948:4:2::19) xfr size: 15 records (envelopes 1)
2019/11/23 17:26:26 cv. cv01.dns.pt. (2a04:6d80::18) xfr size: 5954 records (envelopes 13)
2019/11/23 17:26:26 cv. cv01.dns.pt. (185.39.208.18) xfr size: 5954 records (envelopes 13)
2019/11/23 17:26:26 xn--ogbpf8fl. ns1.tld.sy. (82.137.200.85) xfr size: 251 records (envelopes 1)
2019/11/23 17:26:32 mw. chambo.sdnp.org.mw. (41.221.99.135) xfr size: 3614 records (envelopes 3)
2019/11/23 17:26:32 mw. domwe.sdn.mw. (41.87.5.162) xfr size: 3614 records (envelopes 3)
2019/11/23 17:26:32 mw. domwe.sdn.mw. (196.45.190.9) xfr size: 3614 records (envelopes 3)
2019/11/23 17:26:38 er. sawanew.noc.net.er. (196.200.96.1) xfr size: 26 records (envelopes 1)
2019/11/23 17:26:38 er. zaranew.noc.net.er. (196.200.96.2) xfr size: 42 records (envelopes 1)
2019/11/23 17:26:42 tj. ns2.tojikiston.com. (193.111.11.4) 
 ..........
... SNIP ...
 ..........
2019/11/23 17:27:41 mr. ns-mr.nic.tn. (41.228.63.70) xfr size: 1894 records (envelopes 4)
2019/11/23 17:28:37 xn--90ae. d.nic.bg. (194.0.32.1) xfr size: 7121 records (envelopes 7)
2019/11/23 17:28:37 xn--90ae. d.nic.bg. (2001:678:3c::1) xfr size: 7121 records (envelopes 7)
2019/11/23 17:28:38 bd. surma.btcl.net.bd. (203.112.194.232) xfr size: 52 records (envelopes 1)
2019/11/23 17:28:48 ao. ns02.dns.ao. (197.149.149.11) xfr size: 2179 records (envelopes 2)
2019/11/23 17:28:51 rw. ns1.ricta.org.rw. (196.49.7.188) xfr size: 7607 records (envelopes 6)
2019/11/23 17:28:51 rw. ns3.ricta.org.rw. (41.138.85.98) xfr size: 7607 records (envelopes 6)
2019/11/23 17:28:59 pg. ns1.tiare.net.pg. (202.165.192.23) xfr size: 5132 records (envelopes 12)
2019/11/23 17:29:13 sv. cir.red.sv. (168.243.254.1) xfr size: 22159 records (envelopes 16)
```

## Usage

```
Usage of ./allxfr:
  -ns string
        initial nameserver to use to get the root
  -out string
        directory to save found zones in (default ".")
  -parallel uint
        number of parallel zone transfers to perform (default 10)
  -verbose
        enable verbose output
```

## Building

```
go build
```
