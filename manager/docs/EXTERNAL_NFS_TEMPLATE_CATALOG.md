# External NFS template catalog

This workflow makes a pre-existing QCOW2 file selectable in VM Alpha Manager. It
registers catalog metadata only; it never creates a VM, copies an image, changes
a disk, or starts an import job.

1. Register and verify an External NFS datastore through the native host workflow.
2. An administrator places the image below that datastore's `templates/` directory,
   for example `templates/rocky9-base.qcow2`.
3. Call `POST /api/v1/storage/templates/import` with the registered `datastore_id`,
   template `name`, and relative `path`.

The endpoint permits only a regular `.qcow2` file directly below `templates/`.
It opens the file through a descriptor rooted at the verified NFS mount, uses
`qemu-img info --output=json` on that inherited descriptor, requires `format`
`qcow2` and a positive virtual size, rejects a backing-file chain, and records a
SHA-256 checksum, virtual disk size, datastore ID, and datastore-relative path.
The stored image reference is `datastore://<id>/templates/<file>`; it is not a
host path.

`GET /api/v1/storage/templates` lists the resulting catalog for VM creation.
Selecting one in the VM form only includes its catalog ID in a later VM-create
request. The actual image-clone/import implementation must independently resolve
and revalidate the stored datastore reference before it creates a VM.
