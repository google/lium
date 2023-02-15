# SSH watcher

## How to use

Have you ever had issues trying to port-forward ssh connection to your device?
Did you have to set up the connection again when device reboots? This tool might
be for you. This tool tries to keep ssh connections to devices alive. Useful
when your chroot does not have complicated set up available outside of your
chroot, and your DUT is not available directly via ssh.

To run:

```shell
$ go run sshwatcher.go host port host port host port
```

to keep ssh connection to host with local port forwarded to port 22.

An example:

```shell
go run sshwatcher.go cheeps 2226 eve 2227 kukui 2228 rammus 2229
```

Your ssh config needs to be set up such that interactive password input is not
always required. For DUTs this means use of `testing_rsa` key. See
https://chromium.googlesource.com/chromiumos/docs/+/HEAD/tips-and-tricks.md#How-to-SSH-to-DUT-without-a-password.

Outside of your chroot you will be using a `.ssh/config` like

```
host your-dut-name
    HostName [your complex config comes here]
    ProxyCommand [your extra complex config comes here]
    ControlMaster auto
    ControlPersist 3600
    ControlPath /tmp/ssh-%r@%h:%p
    ServerAliveCountMax 10
    ServerAliveInterval 1
    VerifyHostKeyDNS no
    CheckHostIP no
    UserKnownHostsFile /dev/null
    Compression yes
    IdentityFile ~/.ssh/testing_rsa
```

Inside of your chroot you will be using:

```shell
(chroot)$ ssh localhost -p xxxx  # to ssh connection.
(chroot)$ adb connect localhost:xxxx  # to Chrome OS Android instance.
(chroot)$ tast run localhost:xxxx arc.Boot  # run a tast test
```

See other docs for more examples:

-   [faft tests utilizing servo](https://chromium.googlesource.com/chromiumos/third_party/autotest/+/HEAD/docs/faft-how-to-run-doc.md#running-against-duts-with-tunnelled-ssh)
-   (Googlers only): go/arc-wfh#example-commands

Having `.ssh/config` entry such as below might be helpful to use from inside the
chroot:

```
host your-dut-name.local
    HostName 127.0.0.1
    Port xxxx
```