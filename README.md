# ALL XFR

### AXFR all the things!

<img src="https://user-images.githubusercontent.com/164192/69487828-764bc280-0e15-11ea-8f67-41dd27ac32d8.jpg" width="400" height="300">

This tool performs a [zone transfer (AXFR)](https://en.wikipedia.org/wiki/DNS_zone_transfer) against the root zone servers to obtain the [root zone file](https://www.iana.org/domains/root/files). And then attempts opportunistic zone transfers for every IP for every nameserver for a given zone. Additionally, each NS/A/AAAA record is also re-queried to find additional servers or IPs not included as root glue.

Most zones do not allow zone transfers, however a few do. Sometimes only on a single IP for a given nameserver and not the others, and sometimes only for servers or IPs that are authorative but not incluided in the root zones. This tool will try them all and save every successful transfer.

This tool works best on an IPv4/IPv6 dual stack internet connection.

Providing a zone file with the `-zonefile` flag will attempt a transfer with the domains and sub-domains in the zone file provided.

TLDs in the [Public Suffix List](https://publicsuffix.org/) can be attempted as well with the `-psl` flag.

## Running with a resolver

When running allxfr with a fully recursive caching resolver like BIND/named or Unbound additional zones may be found. You can enable this behavior with the `-ns` flag.

An example Docker configureation for Unbound is provided in the `unbound/` directory, and can be build with `make docker-unbound` and run with `make run-unbound`.


## Example

```
./allxfr -dry-run
ROOT g.root-servers.net. xfr size: 22017 records in 1.334s 
mr. ns-mr.nic.tn. (41.228.63.70) xfr size: 444 records in 337ms
sl. ns1.neoip.com. (45.83.41.38) xfr size: 455 records in 592ms
sy. ns1.tld.sy. (82.137.200.85) xfr size: 1594 records in 870ms
cd. ns-root-2.scpt-network.com. (41.79.235.67) xfr size: 497 records in 598ms
my. a.nic.my. (103.44.108.53) xfr size: 1592 records in 608ms
mq. ns1-mq.mediaserv.net. (213.16.20.3) xfr size: 1541 records in 299ms
td. nsa.nic.td. (154.68.159.246) xfr size: 492 records in 1.821s
gp. ns2.nic.gp. (193.218.114.34) xfr size: 1578 records in 195ms
xn--54b7fta0cc. bayanno.btcl.net.bd. (180.211.212.213) xfr size: 915 records in 728ms
xn--ogbpf8fl. ns1.tld.sy. (82.137.200.85) xfr size: 271 records in 460ms
xn--90ae. a.nic.bg. (2a02:6a80::192:92:129:99) xfr size: 305 records in 326ms
cv. cv01.dns.pt. (185.39.208.18) xfr size: 507 records in 333ms
bd. surma.btcl.net.bd. (203.112.194.232) xfr size: 52 records in 486ms
rw. ns1.ricta.org.rw. (196.49.7.188) xfr size: 1515 records in 1.269s
gf. ns1-mq.mediaserv.net. (213.16.20.3) xfr size: 1531 records in 304ms
bn. ns2.bnnic.bn. (202.93.214.163) xfr size: 79 records in 395ms
tj. ns2.tojikiston.com. (193.111.11.4) xfr size: 1690 records in 998ms
pg. ns1.tiare.net.pg. (202.165.192.23) xfr size: 447 records in 409ms
mw. domwe.sdn.mw. (41.87.5.162) xfr size: 1538 records in 938ms
xn--j1amh. tier1.num.net.ua. (193.110.163.134) xfr size: 1055 records in 1.349s
sv. cir.red.sv. (168.243.254.1) xfr size: 1514 records in 883ms
21 / 1516 transfered in 3m29.92
```

## Usage

```
Usage of ./allxfr:
  -dry-run
        only test if xfr is allowed by retrieving one envelope
  -ixfr
        attempt an IXFR instead of AXFR
  -ns string
        nameserver to use for manualy querying of records not in zone file
  -out string
        directory to save found zones in (default "zones")
  -parallel uint
        number of parallel zone transfers to perform (default 10)
  -psl
        attempt AXFR from zones listed in the public suffix list, requires -ns flag
  -save-all
        attempt AXFR from every nameserver for a given zone and save all answers
  -verbose
        enable verbose output
  -zonefile string
        use the provided zonefile instead of getting the root zonefile
```

## Building

```
go build
```
