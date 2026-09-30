# Guest images

`tt image recipes` lists the twelve recipes from the pinned Rust release.
`tt image create NAME --image-dir DIRECTORY` builds on the machine running the
CLI; it does not upload to an agent. `tt image list` queries the controller's
current agent catalogs. Prepare the image directory before QEMU image creation.

| Recipes | Behavior |
|---|---|
| `alpine`, `debian`, `ubuntu`, `rockylinux` | Pull pinned container tags and add local aliases; base images need a long-running workload |
| `nginx`, `redis`, `postgres` | Pull workload container tags; PostgreSQL requires its own initialization settings |
| `alpine-cloud` | Alpine 3.21.7 NoCloud qcow2, pinned SHA-512 |
| `debian-cloud`, `ubuntu-cloud` | Latest Debian 13 / current Ubuntu 24.04 image, verified against upstream SHA-512 / SHA-256 manifests |
| `fc-alpine` | Pinned kernel and Alpine userspace, 128 MiB ext4 root; boot/network check only, no SSH |
| `freebsd-base` | Experimental Jail root from the FreeBSD host's exact RELEASE series |

```sh
sudo mkdir -p /home/ttstack/images
sudo ./bin/tt image create alpine-cloud
sudo ./bin/tt image create all --engine qemu
```

`all` can be filtered by recipe engine. Without a filter it skips Jail on other
systems and Firecracker on FreeBSD. Container recipes prefer Docker, then Podman.
QEMU recipes need `qemu-img`; raw downloads are converted to qcow2. Downloads are
staged, checksum-verified and atomically published. A failed download or checksum
never replaces the prior image. Existing QEMU images must be nonempty qcow2 disks.

The Firecracker recipe needs Linux mount privileges, `dd`, `mkfs.ext4` and `tar`.
It unmounts even after population failure; an uncertain unmount retains the mount
and staging rootfs for inspection. Do not remove a retained staging directory
until its mount is resolved. A custom Firecracker image must provide the guest
configuration and SSH support required by the workload; `fc-alpine` is not such
an SSH image.

`freebsd-base` uses native `fetch` and `tar`, verifies essential base files and
writes a completion marker before publication. It refuses an incomplete existing
root and non-RELEASE hosts. Image preparation does not by itself establish working
Jail networking or supported guest resource enforcement.

File and zvol storage layouts and runtime paths follow
[compatibility.md](compatibility.md). ZFS datasets and image import are operator
preparation, separate from these local file recipes.
