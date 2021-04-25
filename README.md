# ARPmon
A REST service that monitors the local area network (LAN) using ARP probes.

## Creating the REST Service:

To start up the REST service, build the container and run it using the steps below.

Before doing these steps you may optionally configure your environment with the network `CIDR` you are interested in scanning (see the Notes section below for more on that), and this scanning host's `IPV4` and `MAC` address. The container will use the values you see when you run `make chkvars` (either the ones you have set or the defaults that the Makefile discovers by examining your *default route*.

You may also choose to alter the base URL for the REST service, the host port it binds to, and the number of goroutines the server will spawn.

In general more goroutines is better for faster convergence to discovery of the full set of MACs on your LAN, but there is a tradeoff. More goroutines will create greater load on the host where you run this. In my home I run this on a Raspberry Pi 3B+, running the Desktop Raspberry Pi OS, with 8 goroutines, and the load average tends to hover around between 0.4 and 1.4, which most of the time keeps the machine fairly responsive for other tasks. Most of the time it is well under 1.0.  I have run it with 16 goroutines which is usually okay too but it periodically spikes to load averages like 4 or 5. So I prefer to keep it at 8. If you reduce it to 4 then I think it won't exceed load averages of 1.0.

To try to improve performance you can try modifying the constants at the top of `src/main.go` to experiment with different factors as well.

Here are the steps to build and run the REST server:

```
make build
make run
```

## Using the REST Service:

To get a list of all of the MAC addresses discovered on the LAN, run:

```
make macs
```

To receive a Comma-Separated-Values (CSV) format list list of IP addresses and corresponding MAC addresses, run:

```
make csv
```

If you prefer to receive the data in JSON form, you can run:

```
make check
```

Of course, all of these `make` targets just use `curl` to connect to the REST API provided by this service. Look at the Makefile to see how each of these targets in implements (each is just a single line). Essentially just append "macs", "csv" or "json" to the BASE_URL when you connect to the API to select your preferred output format.

## Notes and limitations

I wrote this new ARPmon code to replace my previous LAN scanning code in [LAN2json](https://github/com/MegaMosquito/LAN2json). That previous code uses the standard `nmap` utility to scan the LAN and it is both slow and unreliable. Often it would never get around to discovering some of the hosts on my LAN. That was unacceptable.

The ARPmon code instead relies upon the [arping Go library](https://pkg.go.dev/github.com/j-keck/arping). It's pretty good, but it has some limitations (see below) that cause it to obfuscate the true MAC address of some of the hosts on the LAN, although I think it will find every single IP address that is in use on your LAN. That is, I think it won't miss any LAN hosts, but it might give incorrect MAC addresses for some, which will make it a little more difficult to know which hardware is using those addresses. That's not a deal-breaker for me.

You must configure ARPmon with the Classless Inter-Domain Routing (CIDR) address of your LAN. For a typical household LAN, with an 8-bit subnet, you will have a network mask of `/24` (or `255.255.255.0`). Your CIDR address can be formed by taking any address on your LAN, removing the last octet (the last number in the IPv4 address) and replacing it with `0/24`. For example, I run this on a machine with IPv4 address `192.168.123.12` and therefore my CIDR is set to `192.168.123.0/24`. There is code in the Makefile to figure out your CIDR for you, but you are best to verify that it did so correctly, by running `make chkvars`. If you have multiple NICs on your host, it might get confused. You can override the computed value by setting CIDR in your environment.

ARPmon will scan every IP address from `.1` to `.254` (inclusive) in the subnet identified by the CIDR you provide. It does this by firing off an **ARP Probe** for each of these addresses. An ARP Probe essentially says, "hey, do any of you computers on the LAN have this IP address? If so, please respond with your MAC address so I can communicate with you". Computers on IP LANs must respond to such messages or other computers using IP will be unable to communicate with them through the lower layers of the protocol stack. That makes this a good technique to find all the hosts on your LAN, at least in principle.

Having said that, ARP is a tricky business, and the underlying `arping` Go library has some limitations as I suggested above. If I understand what I am seeing from its output, I think `arping` does not handle **ARP Proxing** correctly. As a result, some hosts respond as proxies for others, responding to my ARP Probes with something like, "yeah, I know who has that IP address, it's in my ARP table, so I'll share that with you on their behalf." The library I am using here returns those responses with the MAC address of the proxy responder. As a result, I see a some MAC addresses showing up multiple times.

I may be able to improve on this, but it's an improvement over my older code so it's "good enough" for my use case as is.

If you have feedback, or wish to contribute improvements, PRs are welcome!



