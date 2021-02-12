#!/bin/sh

CONTROLLER_GEN_TMP_DIR=$(mktemp -d)
cd $CONTROLLER_GEN_TMP_DIR

go mod init tmp
echo "Downloading controller gen"
go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1
echo "Removing temp dir"
rm -rf $CONTROLLER_GEN_TMP_DIR

echo "Controller gen installed at $(which controller-gen)"
