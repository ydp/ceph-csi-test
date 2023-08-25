# ceph-csi-test

This repo is used to test all GA and Beta features in https://github.com/ceph/ceph-csi/tree/devel#ceph-csi-features-and-available-versions. 

We might also add cases for other features.

This is useful for validating ceph-csi working proper in a new environment.

## Prerequisites

You need to install both rbd and cephfs plugins to run cases in this repo.

Cases:

```
Rbd [GA] [PVC] should be able to dynamically provision Block mode RWO volume
Rbd [GA] [PVC] should be able to dynamically provision Block mode RWX volume
Rbd [GA] [PVC] should be able to dynamically provision File mode RWO volume
Rbd [GA] [clone] should be able to provision File mode RWO volume from another volume
Rbd [GA] [clone] should be able to provision Block mode RWO volume from another volume
Rbd [GA] [metrics] should be able to collect metrics of Block mode volume
Rbd [GA] [metrics] should be able to collect metrics of File mode volume
Rbd [GA] [snapshot] should be able to provision volume from snapshot
Rbd [Beta] should be able to expand volume
Cephfs [GA] [PVC] should be able to dynamically provision File mode RWO volume
Cephfs [GA] [PVC] should be able to dynamically provision File mode RWX volume
Cephfs [GA] [clone] should be able to provision volume from another volume
Cephfs [GA] [metrics] should be able to collect metrics of File mode volume
Cephfs [GA] [snapshot] should be able to provision volume from snapshot
Cephfs [Beta] should be able to expand volume
```
