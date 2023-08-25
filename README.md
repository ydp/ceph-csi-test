# ceph-csi-test

This repo is used to test all GA and Beta features in https://github.com/ceph/ceph-csi/tree/devel#ceph-csi-features-and-available-versions. 

We might also add cases for other features.

This is useful for validating ceph-csi working proper in a new environment.

## Prerequisites

* Install both rbd and cephfs plugins via rook connecting to external ceph cluster
  * rook needs to install in `rook-ceph` namespace
  * the cluster id is named as `rook-ceph-external`
* Use below command to create users that is for access ceph cluster from K8s

```
ceph auth get-or-create client.csi-rbd-node \
mon 'profile rbd' \
osd 'profile rbd' \
mgr 'allow rw'

ceph auth get-or-create client.csi-rbd-provisioner \
mon 'profile rbd' \
osd 'profile rbd' \
mgr 'allow rw'

ceph auth get-or-create client.csi-cephfs-node \
mon 'allow r' \
osd 'allow rw tag cephfs metadata=*' \
mgr 'allow rw'

ceph auth get-or-create client.csi-cephfs-provisioner \
mon 'allow r' \
osd 'allow rw tag cephfs *=*' \
mgr 'allow rw' \
mds 'allow rw'
```

* Below secrets needs to exist in `rook-ceph-external` namespace
  * rook-csi-rbd-node

```
data:
  userID: Y3NpLXJiZC1ub2Rl
  userKey: QVFDOEcrVmtQUjNlQ0JBQUV5SE5LZ2pvQjFCaGxWSmRKQlYxM0E9PQ==
```

  * rook-csi-rbd-provisioner

```
data:
  userID: Y3NpLXJiZC1wcm92aXNpb25lcg==
  userKey: QVFDN0crVmtTeGNBTkJBQU00MW5ON1NsREE2VU5nNldkTmxGRnc9PQ==
```

  * rook-csi-cephfs-node

```
data:
  adminID: Y3NpLWNlcGhmcy1ub2Rl
  adminKey: QVFDOEcrVmtyRTdoTFJBQUtoT0lsTmJyY3drWXNKM3poa1NaaFE9PQ==
```

  * rook-csi-cephfs-provisioner

```
data:
  adminID: Y3NpLWNlcGhmcy1wcm92aXNpb25lcg==
  adminKey: QVFDOEcrVmtDQ3VNSEJBQVdzMmQxVGlrRTQ4b2NWOXAvMGovTHc9PQ==
```
* The machine that running the cases needs to have access to ceph cluster, since we need to validate data from ceph side

Cases:

```
Rbd [GA] [PVC] should be able to dynamically provision Block mode RWO volume [pvc]
Rbd [GA] [PVC] should be able to dynamically provision Block mode RWX volume [rdb, pvc, rwx]
Rbd [GA] [PVC] should be able to dynamically provision File mode RWO volume [pvc]
Rbd [GA] [PVC] [clone] should be able to provision File mode RWO volume from another volume [clone]
Rbd [GA] [PVC] [clone] should be able to provision Block mode RWO volume from another volume [clone]
Rbd [GA] [PVC] should be able to collect metrics of Block mode volume [metrics]
Rbd [GA] [PVC] should be able to collect metrics of File mode volume [metrics]
Rbd [GA] [snapshot] should be able to provision volume from snapshot [snapshot]
Rbd [Beta] should be able to expand volume [rbd, beta, expansion]
Cephfs [GA] [PVC] should be able to dynamically provision File mode RWO volume [pvc]
Cephfs [GA] [PVC] should be able to dynamically provision File mode RWX volume [cephfs, pvc, rwx]
Cephfs [GA] [PVC] [clone] should be able to provision volume from another volume [clone]
Cephfs [GA] [PVC] should be able to collect metrics of File mode volume [metrics]
Cephfs [GA] [snapshot] should be able to provision volume from snapshot [snapshot]
Cephfs [Beta] should be able to expand volume [cephfs, beta, expansion]
```

Latest result:

```
Ran 12 of 15 Specs in 468.273 seconds
SUCCESS! -- 12 Passed | 0 Failed | 0 Pending | 3 Skipped
PASS
```
