## Blocking Guest -> Host communication

As of now, the guest can easily access any networked service running on the host via the TAP device attached to the VM

On the host:

```sh
$ ip a
...
16: hypercore-0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP group default qlen 1000
    link/ether 62:56:73:02:94:a2 brd ff:ff:ff:ff:ff:ff
    inet 169.254.0.2/30 brd 169.254.0.3 scope global hypercore-0
       valid_lft forever preferred_lft forever
```

In the guest:

```sh
$ ip a
...
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
    link/ether 06:00:ac:10:00:02 brd ff:ff:ff:ff:ff:ff
    inet 169.254.0.1/30 brd 169.254.0.3 scope global eth0
       valid_lft forever preferred_lft forever
$ ip route
default via 169.254.0.2 dev eth0 
169.254.0.0/30 dev eth0 scope link  src 169.254.0.1 
```

Via the route `169.254.0.2`, the guest can access any port on the host, and similarly, the host can access any exposed port on the guest

We only desire the latter though, but I've not been able to find any straightforward way of configuring the network accordingly

## References

- [https://github.com/moby/moby/tree/master/libnetwork](libnetwork) has a `bridge` driver that seems to accomplish our goals (Host -> Container, but not Container -> Host), but I was not able to find anything obvious in their bridge network setup, or any of the IPTables rules that are set up

- [https://www.cni.dev/plugins/current/meta/firewall](firewall) CNI plugin, did not find any example of projects using this for the use-case mentioned here

- [https://github.com/awslabs/tc-redirect-tap](tc-redirect-tap)

- [https://github.com/firecracker-microvm/firecracker-go-sdk](firecracker-go-sdk) contains integration with CNI plugins, can be used as an example of using them
