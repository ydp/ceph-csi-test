# ceph-csi-test

This repo is used to test all GA and Beta features in https://github.com/ceph/ceph-csi/tree/devel#ceph-csi-features-and-available-versions. 

We might also add cases for other features.

This is useful for validating ceph-csi working proper in a new environment.

## Prerequisites

* Install both rbd and cephfs plugins via rook (version `v1.11.10`) connecting to external ceph cluster
  * rook needs to install in `rook-ceph` namespace
  * the cluster id is named as `rook-ceph-external`
* Use below command to create users that is for access ceph cluster from K8s

```
ceph auth get-or-create client.healthchecker \
mon 'allow r, allow command quorum_status, allow command version' \
osd 'allow rwx pool=default.rgw.meta, allow r pool=.rgw.root, allow rw pool=default.rgw.control, allow rx pool=default.rgw.log, allow x pool=default.rgw.buckets.index' \
mgr 'allow command config'

ceph auth get-or-create client.csi-rbd-provisioner \
mon 'profile rbd' \
osd 'profile rbd' \
mgr 'allow rw'

ceph auth get-or-create client.csi-rbd-node \
mon 'profile rbd' \
osd 'profile rbd' \
mgr 'allow rw'

ceph auth get-or-create client.csi-cephfs-provisioner \
mon 'allow r' \
osd 'allow rw tag cephfs metadata=*' \
mgr 'allow rw'

ceph auth get-or-create client.csi-cephfs-node \
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


Using Pool detail:

```
pool 1 '.mgr' replicated size 3 min_size 2 crush_rule 0 object_hash rjenkins pg_num 1 pgp_num 1 autoscale_mode on last_change 18 flags hashpspool stripe_width 0 pg_num_max 32 pg_num_min 1 application mgr
pool 2 'myfs-metadata' replicated size 3 min_size 2 crush_rule 1 object_hash rjenkins pg_num 16 pgp_num 16 autoscale_mode on last_change 69 lfor 0/0/38 flags hashpspool stripe_width 0 compression_mode none pg_autoscale_bias 4 pg_num_min 16 recovery_priority 5 application cephfs
pool 3 'myfs-replicated' replicated size 3 min_size 2 crush_rule 2 object_hash rjenkins pg_num 32 pgp_num 32 autoscale_mode on last_change 282 lfor 0/0/38 flags hashpspool,selfmanaged_snaps stripe_width 0 compression_mode none application cephfs
pool 4 'replicapool' replicated size 3 min_size 2 crush_rule 3 object_hash rjenkins pg_num 32 pgp_num 32 autoscale_mode on last_change 298 lfor 0/0/40 flags hashpspool,selfmanaged_snaps stripe_width 0 compression_mode none application rbd
```

Cases:

```
Rbd [GA] should be able to dynamically provision Block mode RWO volume [rbd, rwo, block]
Rbd [GA] should be able to dynamically provision Block mode RWX volume [rbd, rwx, block]
Rbd [GA] should be able to dynamically provision File mode RWO volume [rbd, rwo, file]
Rbd [GA] should be able to provision File mode RWO volume from another volume [rbd, clone, file]
Rbd [GA] should be able to provision Block mode RWO volume from another volume [rbd, clone, block]
Rbd [GA] should be able to create ephemeral File mode volume [rbd, ephemeral, file]
Rbd [GA] should be able to create ephemeral Block mode volume [rbd, ephemeral, block]
Rbd [GA] should be able to create statefulset w/ File mode volume [rbd, statefulset, file]
Rbd [GA] should be able to create statefulset w/ Block mode volume [rbd, statefulset, block]
Rbd [GA] should be able to collect metrics of Block mode volume [rbd, metrics, block]
Rbd [GA] should be able to collect metrics of File mode volume [rbd, metrics, file]
Rbd [GA] should be able to provision File volume from snapshot [rbd, snapshot, file]
Rbd [GA] should be able to provision Block volume from snapshot [rbd, snapshot, block]
Rbd [Beta] should be able to expand volume [rbd, expansion, file]
Rbd [Beta] should be able to expand volume [rbd, expansion, block]
Cephfs [GA] should be able to dynamically provision File mode RWO volume [cephfs, pvc, rwo]
Cephfs [GA] should be able to dynamically provision File mode RWX volume [cephfs, pvc, rwx]
Cephfs [GA] should be able to provision volume from another volume [cephfs, clone]
Cephfs [GA] should be able to collect metrics of File mode volume [cephfs, metrics]
Cephfs [GA] should be able to provision volume from snapshot [cephfs, snapshot]
Cephfs [Beta] should be able to expand volume [cephfs, beta, expansion]
```

Latest result:

```
Ran 15 of 21 Specs in 922.397 seconds
SUCCESS! -- 15 Passed | 0 Failed | 0 Pending | 6 Skipped
PASS
```
