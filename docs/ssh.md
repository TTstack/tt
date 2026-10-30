# Initial guest SSH access

QEMU cloud images and prepared Firecracker images accept initial public keys:

```sh
tt env create work --image alpine-cloud --engine qemu \
  --cpu 1 --mem 512 --lifetime 0 \
  --ssh-key ~/.ssh/id_ed25519.pub --ssh-user user --ssh-sudo
tt env show work
```

The account defaults to `root`; `--ssh-sudo` grants unrestricted guest sudo.
Accounts must start with a lowercase letter and contain at most 32 lowercase
letters, digits, `_` or `-`. Explicit SSH options require a public key. TTstack
does not accept the caller's login private key and has no key-update API.

QEMU needs cloud-init and OpenSSH. Firecracker needs account/OpenSSH tools, an
ext4 root containing `/etc/ttstack/ssh-bootstrap-version` with content `1`, and an
init that mounts the read-only configuration drive and executes `ttstack-ssh.sh`
before starting sshd. That seed filename is reserved. Sudo also needs
`sudo`/`visudo`. The built-in `fc-alpine` recipe does not provide managed SSH. The prepared kernel
must also supply usable entropy; the native SSH tests use Firecracker’s 5.10.233
CI kernel with `CONFIG_HW_RANDOM_VIRTIO=y`.

Provisioning creates an independent Ed25519 guest host identity and key-only
login. `/var/lib/ttstack/ssh-initialized` records successful initial provisioning.
Normal restart retains the disk, keys and guest edits; it does not rotate keys or
restore removed authorized keys. Images must not contain an initialization marker
or shared private keys. Retained host seeds and QEMU ISOs are private 0600 files.

`tt-agent --ssh-public-address IPv4` supplies the client-visible endpoint. An
empty advertised host means public access was not configured. Readiness is an
observation from the agent, not proof of external routing. Initial readiness
checks the provisioned host identity; later checks observe the SSH banner.

For a private Linux agent network, `--ssh-ingress-netns` and
`--ssh-ingress-target` must be supplied together with the public address. These
install only owned SSH rules in the specified outer network namespace. Operators
prepare routing and nonoverlapping `--port-start`/`--port-end` ranges. The agent
persists ingress and port allocation settings and refuses changes while VM
records remain. In test fleets, the outer namespace must itself be task-owned;
never point tests at the shared host namespace.

FreeBSD Jail, validated on FreeBSD 15.x only, supports only root public-key injection with default
root/no-sudo options and has no managed SSH response metadata. Bhyve has no key
injection. Docker/Podman has no managed SSH bootstrap.

Disk restore clears the agent's SSH readiness observations and requires the
initial identity to be observed again after boot. It does not force guest
provisioning or adopt a changed host key. See [disk-backup.md](disk-backup.md) and
[validation records](validation/) for tested behavior and remaining native cases.
